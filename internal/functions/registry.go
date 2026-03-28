// Package functions provides a registry for SPL 2.0 user-defined functions,
// procedures, and built-in callable tools.
// Mirrors spl/functions.py from the Python SPL runtime.
package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/tokencount"
)

// Registry holds user-defined functions, procedures, and built-in tools.
type Registry struct {
	functions  map[string]*ast.CreateFunctionStatement
	procedures map[string]*ast.ProcedureStatement
}

// New creates a new empty Registry.
func New() *Registry {
	return &Registry{
		functions:  make(map[string]*ast.CreateFunctionStatement),
		procedures: make(map[string]*ast.ProcedureStatement),
	}
}

// RegisterFunction registers an SPL CREATE FUNCTION statement.
func (r *Registry) RegisterFunction(stmt *ast.CreateFunctionStatement) {
	r.functions[stmt.Name] = stmt
}

// RegisterProcedure registers an SPL PROCEDURE statement.
func (r *Registry) RegisterProcedure(stmt *ast.ProcedureStatement) {
	r.procedures[stmt.Name] = stmt
}

// GetFunction returns a registered function by name.
func (r *Registry) GetFunction(name string) (*ast.CreateFunctionStatement, bool) {
	fn, ok := r.functions[name]
	return fn, ok
}

// GetProcedure returns a registered procedure by name.
func (r *Registry) GetProcedure(name string) (*ast.ProcedureStatement, bool) {
	proc, ok := r.procedures[name]
	return proc, ok
}

// IsBuiltin returns true if name is a known built-in callable.
func (r *Registry) IsBuiltin(name string) bool {
	_, ok := builtins[strings.ToLower(name)]
	return ok
}

// CallBuiltin calls a built-in function by name with the given args.
// Returns (result, true) if found, ("", false) if not a built-in.
func (r *Registry) CallBuiltin(name string, args []string) (string, bool) {
	fn, ok := builtins[strings.ToLower(name)]
	if !ok {
		return "", false
	}
	return fn(args), true
}

// builtins maps lowercase function name to implementation.
var builtins = map[string]func([]string) string{
	"summarize":    builtinSummarize,
	"len":          builtinLen,
	"upper":        builtinUpper,
	"lower":        builtinLower,
	"truncate":     builtinTruncate,
	"list":         builtinList,
	"get":          builtinGet,
	"append":       builtinAppend,
	"count":        builtinCount,
	"join":         builtinJoin,
	"list_append":  builtinAppend,
	"list_concat":  builtinListConcat,
	"list_count":   builtinCount,
	"list_get":     builtinGet,
	"write_file":   builtinWriteFile,
	"read_file":    builtinReadFile,
}

// builtinSummarize extracts approximately maxTokens worth of sentences from text.
// Usage: summarize(text, maxTokens)
func builtinSummarize(args []string) string {
	if len(args) == 0 {
		return ""
	}
	text := args[0]
	maxTokens := 200
	if len(args) >= 2 {
		n := 0
		fmt.Sscanf(args[1], "%d", &n)
		if n > 0 {
			maxTokens = n
		}
	}
	c := tokencount.New("gpt")
	return c.TruncateToTokens(text, maxTokens)
}

// builtinLen returns the token count estimate of text as a string.
// Usage: len(text)
func builtinLen(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	c := tokencount.New("gpt")
	return fmt.Sprintf("%d", c.Count(args[0]))
}

// builtinUpper converts text to uppercase.
func builtinUpper(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToUpper(args[0])
}

// builtinLower converts text to lowercase.
func builtinLower(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToLower(args[0])
}

// builtinTruncate truncates text to maxChars characters, appending "..." if truncated.
// Usage: truncate(text, maxChars)
func builtinTruncate(args []string) string {
	if len(args) == 0 {
		return ""
	}
	text := args[0]
	maxChars := len(text)
	if len(args) >= 2 {
		n := 0
		fmt.Sscanf(args[1], "%d", &n)
		if n > 0 {
			maxChars = n
		}
	}
	if len(text) <= maxChars {
		return text
	}
	if maxChars > 3 {
		return text[:maxChars-3] + "..."
	}
	return text[:maxChars]
}

// builtinList creates a JSON array string from args.
// Usage: list(item1, item2, ...)
func builtinList(args []string) string {
	data, _ := json.Marshal(args)
	return string(data)
}

// builtinGet retrieves an element from a JSON array or comma-separated list by index.
// Usage: get(collection, index)
func builtinGet(args []string) string {
	if len(args) < 2 {
		return ""
	}
	collection := args[0]
	idx := 0
	fmt.Sscanf(args[1], "%d", &idx)

	// Try JSON array first
	var arr []string
	if json.Unmarshal([]byte(collection), &arr) == nil {
		if idx >= 0 && idx < len(arr) {
			return arr[idx]
		}
		return ""
	}

	// Fall back to comma-separated
	parts := strings.Split(collection, ",")
	if idx >= 0 && idx < len(parts) {
		return strings.TrimSpace(parts[idx])
	}
	return ""
}

// builtinAppend appends an item to a JSON array (or creates one).
// Usage: append(collection, item)
func builtinAppend(args []string) string {
	if len(args) < 2 {
		if len(args) == 1 {
			return builtinList(args)
		}
		return "[]"
	}
	collection := args[0]
	item := args[1]

	var arr []string
	if json.Unmarshal([]byte(collection), &arr) != nil {
		// Not a JSON array — start fresh if empty, else wrap existing
		if strings.TrimSpace(collection) == "" {
			arr = []string{}
		} else {
			arr = []string{collection}
		}
	}
	arr = append(arr, item)
	data, _ := json.Marshal(arr)
	return string(data)
}

// builtinCount returns the length of a JSON array.
// Usage: count(collection)
func builtinCount(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	var arr []string
	if json.Unmarshal([]byte(args[0]), &arr) == nil {
		return fmt.Sprintf("%d", len(arr))
	}
	// Fall back to comma-separated count
	if strings.TrimSpace(args[0]) == "" {
		return "0"
	}
	return fmt.Sprintf("%d", len(strings.Split(args[0], ",")))
}

// builtinJoin joins a JSON array with a separator.
// Usage: join(collection, sep)
func builtinJoin(args []string) string {
	if len(args) == 0 {
		return ""
	}
	collection := args[0]
	sep := ", "
	if len(args) >= 2 {
		sep = args[1]
	}

	var arr []string
	if json.Unmarshal([]byte(collection), &arr) == nil {
		return strings.Join(arr, sep)
	}
	return collection
}

// builtinListConcat concatenates two JSON arrays.
// Usage: list_concat(arr1, arr2)
func builtinListConcat(args []string) string {
	if len(args) == 0 {
		return "[]"
	}
	var result []string
	for _, arg := range args {
		var arr []string
		if json.Unmarshal([]byte(arg), &arr) == nil {
			result = append(result, arr...)
		} else if strings.TrimSpace(arg) != "" {
			result = append(result, arg)
		}
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// builtinWriteFile writes content to a UTF-8 file.
// Usage: write_file(path, content)
func builtinWriteFile(args []string) string {
	if len(args) < 2 {
		return "error: write_file requires path and content"
	}
	path := args[0]
	content := args[1]
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "written: " + path
}

// builtinReadFile reads a UTF-8 file and returns its content.
// Usage: read_file(path)
func builtinReadFile(args []string) string {
	if len(args) == 0 {
		return ""
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return ""
	}
	return string(data)
}
