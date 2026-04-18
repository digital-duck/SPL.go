// Package tools loads Python @spl_tool functions from a .py file and wraps
// them as Go functions callable from the SPL executor's CALL dispatch.
//
// Each discovered tool is invoked as a Python subprocess:
//
//	python3 -c "import sys; sys.path.insert(0,'<dir>'); from <module> import <fn>; ..."
//
// This matches the Python runtime's --tools behaviour exactly.
package tools

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Load parses a Python tools file, finds all @spl_tool-decorated functions,
// and returns a map of function name → Go callable.
//
// The Go callable invokes the Python function via subprocess, passing all
// SPL CALL arguments as positional strings.
func Load(toolsPath string) (map[string]func(args []string) string, error) {
	absPath, err := filepath.Abs(toolsPath)
	if err != nil {
		return nil, fmt.Errorf("tools: cannot resolve path %q: %w", toolsPath, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("tools: file not found: %q", absPath)
	}

	funcNames, err := parseSplToolFunctions(absPath)
	if err != nil {
		return nil, fmt.Errorf("tools: parse error in %q: %w", absPath, err)
	}
	if len(funcNames) == 0 {
		return nil, fmt.Errorf("tools: no @spl_tool functions found in %q", absPath)
	}

	dir := filepath.Dir(absPath)
	module := strings.TrimSuffix(filepath.Base(absPath), ".py")

	tools := make(map[string]func(args []string) string, len(funcNames))
	for _, name := range funcNames {
		fnName := name // capture
		tools[fnName] = func(args []string) string {
			return callPythonTool(dir, module, fnName, args)
		}
	}
	return tools, nil
}

// parseSplToolFunctions scans the Python file for "@spl_tool" annotations
// and returns the names of the functions immediately following them.
func parseSplToolFunctions(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var names []string
	nextIsToolFunc := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "@spl_tool" {
			nextIsToolFunc = true
			continue
		}

		if nextIsToolFunc {
			if strings.HasPrefix(line, "def ") {
				// Extract the function name: "def some_name(..."
				rest := strings.TrimPrefix(line, "def ")
				if idx := strings.IndexAny(rest, "(: "); idx > 0 {
					names = append(names, rest[:idx])
				}
			}
			nextIsToolFunc = false
		}
	}
	return names, scanner.Err()
}

// callPythonTool invokes a single Python tool function via subprocess and
// returns its string output (trimmed). Returns an error string on failure.
//
// A lightweight mock of `spl.tools.spl_tool` is injected into sys.modules
// before the user file is imported so the tools work even when the `spl`
// Python package is not installed in the subprocess's environment.
func callPythonTool(dir, module, fnName string, args []string) string {
	// Bootstrap: mock spl.tools.spl_tool as a no-op decorator so the tools
	// file can be imported without requiring the spl package to be installed.
	bootstrap := "import sys, types; " +
		"_m = types.ModuleType('spl'); _m.tools = types.ModuleType('spl.tools'); " +
		"_m.tools.spl_tool = lambda f: f; " +
		"sys.modules.setdefault('spl', _m); sys.modules.setdefault('spl.tools', _m.tools); "

	snippet := fmt.Sprintf(
		"%ssys.path.insert(0, %q); from %s import %s; "+
			"result = %s(*sys.argv[1:]); print('' if result is None else str(result))",
		bootstrap, dir, module, fnName, fnName,
	)

	cmdArgs := append([]string{"-c", snippet}, args...)
	cmd := exec.Command("python3", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("tool_error: %s: %s", fnName, strings.TrimSpace(string(ee.Stderr)))
		}
		return fmt.Sprintf("tool_error: %s: %v", fnName, err)
	}
	return strings.TrimSpace(string(out))
}
