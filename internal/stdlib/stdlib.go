// Package stdlib implements the SPL 2.0 standard library of built-in functions.
package stdlib

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Func is a standard library function: takes string args, returns a string.
type Func func(args []string) string

// Registry holds all registered stdlib functions.
var Registry = map[string]Func{
	// Type conversion
	"to_int":   toInt,
	"to_float": toFloat,
	"to_text":  toText,
	"to_bool":  toBool,

	// String
	"upper":      upper,
	"lower":      lower,
	"trim":       trim,
	"ltrim":      ltrim,
	"rtrim":      rtrim,
	"length":     length,
	"substr":     substr,
	"replace":    replace,
	"concat":     concat,
	"instr":      instr,
	"lpad":       lpad,
	"rpad":       rpad,
	"split_part": splitPart,
	"reverse":    reverse,

	// Pattern matching
	"like":         like,
	"startswith":   startswith,
	"endswith":     endswith,
	"contains":     contains,
	"regexp_match": regexpMatch,

	// Numeric
	"abs_val":   absVal,
	"round_val": roundVal,
	"ceil_val":  ceilVal,
	"floor_val": floorVal,
	"mod_val":   modVal,
	"power_val": powerVal,
	"sqrt_val":  sqrtVal,
	"sign_val":  signVal,
	"clamp":     clamp,

	// Conditional
	"coalesce": coalesce,
	"nullif":   nullif,
	"iif":      iif,

	// Null/empty
	"isnull":  isnull,
	"nvl":     nvl,
	"isblank": isblank,

	// Text aggregate
	"word_count": wordCount,
	"char_count": charCount,
	"line_count": lineCount,

	// JSON
	"json_get":    jsonGet,
	"json_set":    jsonSet,
	"json_keys":   jsonKeys,
	"json_length": jsonLength,
	"json_pretty": jsonPretty,

	// Date/time
	"now_iso":         nowISO,
	"date_format_val": dateFormatVal,
	"date_diff_days":  dateDiffDays,

	// Hashing
	"md5_hash":    md5Hash,
	"sha256_hash": sha256Hash,

	// List
	"list_get":      listGet,
	"list_length":   listLength,
	"list_join":     listJoin,
	"list_contains": listContains,
	"trim_turns":    trimTurns,
	"count":         listLength, // COUNT(@list) — alias for list_length; case-folded by stdlib.Call
	"len_val":       lenVal,     // polymorphic length: string chars, JSON array elements, JSON object keys
}

// Call invokes a stdlib function by name. Returns (result, ok).
func Call(name string, args []string) (string, bool) {
	fn, ok := Registry[strings.ToLower(name)]
	if !ok {
		return "", false
	}
	return fn(args), true
}

// Has checks if a function exists in the stdlib.
func Has(name string) bool {
	_, ok := Registry[strings.ToLower(name)]
	return ok
}

// =============================================================================
// Type conversion
// =============================================================================

func toInt(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	v := strings.TrimSpace(args[0])
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return "0"
	}
	return strconv.Itoa(int(f))
}

func toFloat(args []string) string {
	if len(args) == 0 {
		return "0.0"
	}
	v := strings.TrimSpace(args[0])
	re := regexp.MustCompile(`-?\d+(?:\.\d+)?`)
	m := re.FindString(v)
	if m == "" {
		return "0.0"
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return "0.0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func toText(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func toBool(args []string) string {
	if len(args) == 0 {
		return "false"
	}
	v := strings.ToLower(strings.TrimSpace(args[0]))
	switch v {
	case "1", "true", "yes", "on", "t", "y":
		return "true"
	}
	return "false"
}

// =============================================================================
// String functions
// =============================================================================

func upper(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToUpper(args[0])
}

func lower(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToLower(args[0])
}

func trim(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[0])
}

func ltrim(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimLeft(args[0], " \t\r\n")
}

func rtrim(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimRight(args[0], " \t\r\n")
}

func length(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	return strconv.Itoa(len(args[0]))
}

func substr(args []string) string {
	if len(args) < 2 {
		return ""
	}
	s := args[0]
	start, err := strconv.Atoi(args[1])
	if err != nil {
		return ""
	}
	// 1-based indexing
	start--
	if start < 0 {
		start = 0
	}
	if start >= len(s) {
		return ""
	}

	if len(args) >= 3 {
		l, err := strconv.Atoi(args[2])
		if err != nil {
			return s[start:]
		}
		end := start + l
		if end > len(s) {
			end = len(s)
		}
		return s[start:end]
	}
	return s[start:]
}

func replace(args []string) string {
	if len(args) < 3 {
		return safeArg(args, 0)
	}
	return strings.ReplaceAll(args[0], args[1], args[2])
}

func concat(args []string) string {
	return strings.Join(args, "")
}

func instr(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	idx := strings.Index(args[0], args[1])
	if idx < 0 {
		return "0"
	}
	return strconv.Itoa(idx + 1) // 1-based
}

func lpad(args []string) string {
	if len(args) < 3 {
		return safeArg(args, 0)
	}
	s := args[0]
	width, err := strconv.Atoi(args[1])
	if err != nil {
		return s
	}
	fill := args[2]
	if fill == "" {
		fill = " "
	}
	for len(s) < width {
		s = fill + s
	}
	return s
}

func rpad(args []string) string {
	if len(args) < 3 {
		return safeArg(args, 0)
	}
	s := args[0]
	width, err := strconv.Atoi(args[1])
	if err != nil {
		return s
	}
	fill := args[2]
	if fill == "" {
		fill = " "
	}
	for len(s) < width {
		s = s + fill
	}
	return s
}

func splitPart(args []string) string {
	if len(args) < 3 {
		return ""
	}
	s := args[0]
	delim := args[1]
	part, err := strconv.Atoi(args[2])
	if err != nil {
		return ""
	}
	parts := strings.Split(s, delim)
	// 1-based
	idx := part - 1
	if idx < 0 || idx >= len(parts) {
		return ""
	}
	return parts[idx]
}

func reverse(args []string) string {
	if len(args) == 0 {
		return ""
	}
	runes := []rune(args[0])
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// =============================================================================
// Pattern matching
// =============================================================================

func like(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	s := args[0]
	pattern := args[1]
	// Convert SQL LIKE pattern to regex: % -> .*, _ -> .
	regexStr := "(?i)^" + regexp.QuoteMeta(pattern) + "$"
	regexStr = strings.ReplaceAll(regexStr, "%", ".*")
	regexStr = strings.ReplaceAll(regexStr, "_", ".")
	matched, err := regexp.MatchString(regexStr, s)
	if err != nil {
		return "false"
	}
	if matched {
		return "true"
	}
	return "false"
}

func startswith(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	if strings.HasPrefix(args[0], args[1]) {
		return "true"
	}
	return "false"
}

func endswith(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	if strings.HasSuffix(args[0], args[1]) {
		return "true"
	}
	return "false"
}

func contains(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	if strings.Contains(args[0], args[1]) {
		return "true"
	}
	return "false"
}

func regexpMatch(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	matched, err := regexp.MatchString(args[1], args[0])
	if err != nil {
		return "false"
	}
	if matched {
		return "true"
	}
	return "false"
}

// =============================================================================
// Numeric functions
// =============================================================================

func absVal(args []string) string {
	f := parseFloat(safeArg(args, 0))
	return formatNum(math.Abs(f))
}

func roundVal(args []string) string {
	f := parseFloat(safeArg(args, 0))
	return formatNum(math.Round(f))
}

func ceilVal(args []string) string {
	f := parseFloat(safeArg(args, 0))
	return formatNum(math.Ceil(f))
}

func floorVal(args []string) string {
	f := parseFloat(safeArg(args, 0))
	return formatNum(math.Floor(f))
}

func modVal(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	a := parseFloat(args[0])
	b := parseFloat(args[1])
	if b == 0 {
		return "0"
	}
	return formatNum(math.Mod(a, b))
}

func powerVal(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	base := parseFloat(args[0])
	exp := parseFloat(args[1])
	return formatNum(math.Pow(base, exp))
}

func sqrtVal(args []string) string {
	f := parseFloat(safeArg(args, 0))
	return formatNum(math.Sqrt(f))
}

func signVal(args []string) string {
	f := parseFloat(safeArg(args, 0))
	switch {
	case f > 0:
		return "1"
	case f < 0:
		return "-1"
	default:
		return "0"
	}
}

func clamp(args []string) string {
	if len(args) < 3 {
		return safeArg(args, 0)
	}
	x := parseFloat(args[0])
	lo := parseFloat(args[1])
	hi := parseFloat(args[2])
	if x < lo {
		x = lo
	} else if x > hi {
		x = hi
	}
	return formatNum(x)
}

// =============================================================================
// Conditional
// =============================================================================

func coalesce(args []string) string {
	for _, a := range args {
		if a != "" {
			return a
		}
	}
	return ""
}

func nullif(args []string) string {
	if len(args) < 2 {
		return safeArg(args, 0)
	}
	if args[0] == args[1] {
		return ""
	}
	return args[0]
}

func iif(args []string) string {
	if len(args) < 3 {
		return ""
	}
	cond := strings.ToLower(strings.TrimSpace(args[0]))
	if cond == "true" || cond == "1" || cond == "yes" {
		return args[1]
	}
	return args[2]
}

// =============================================================================
// Null/empty
// =============================================================================

func isnull(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return "true"
	}
	return "false"
}

func nvl(args []string) string {
	if len(args) < 2 {
		return safeArg(args, 0)
	}
	if args[0] == "" {
		return args[1]
	}
	return args[0]
}

func isblank(args []string) string {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "true"
	}
	return "false"
}

// =============================================================================
// Text aggregate
// =============================================================================

func wordCount(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return "0"
	}
	words := strings.Fields(args[0])
	return strconv.Itoa(len(words))
}

func charCount(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	return strconv.Itoa(len(args[0]))
}

func lineCount(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return "0"
	}
	return strconv.Itoa(strings.Count(args[0], "\n") + 1)
}

// =============================================================================
// JSON
// =============================================================================

func jsonGet(args []string) string {
	if len(args) < 2 {
		return ""
	}
	var obj interface{}
	if err := json.Unmarshal([]byte(args[0]), &obj); err != nil {
		return ""
	}
	parts := strings.Split(args[1], ".")
	cur := obj
	for _, part := range parts {
		if part == "" {
			continue
		}
		switch v := cur.(type) {
		case map[string]interface{}:
			cur = v[part]
		default:
			return ""
		}
	}
	if cur == nil {
		return ""
	}
	switch v := cur.(type) {
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func jsonSet(args []string) string {
	if len(args) < 3 {
		return safeArg(args, 0)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(args[0]), &obj); err != nil {
		obj = make(map[string]interface{})
	}
	parts := strings.Split(args[1], ".")
	cur := obj
	for i, part := range parts {
		if i == len(parts)-1 {
			cur[part] = args[2]
		} else {
			if _, ok := cur[part]; !ok {
				cur[part] = make(map[string]interface{})
			}
			if next, ok := cur[part].(map[string]interface{}); ok {
				cur = next
			} else {
				break
			}
		}
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return args[0]
	}
	return string(b)
}

func jsonKeys(args []string) string {
	if len(args) == 0 {
		return "[]"
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(args[0]), &obj); err != nil {
		return "[]"
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	b, _ := json.Marshal(keys)
	return string(b)
}

func jsonLength(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	var obj interface{}
	if err := json.Unmarshal([]byte(args[0]), &obj); err != nil {
		return "0"
	}
	switch v := obj.(type) {
	case map[string]interface{}:
		return strconv.Itoa(len(v))
	case []interface{}:
		return strconv.Itoa(len(v))
	default:
		return "1"
	}
}

func jsonPretty(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var obj interface{}
	if err := json.Unmarshal([]byte(args[0]), &obj); err != nil {
		return args[0]
	}
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return args[0]
	}
	return string(b)
}

// =============================================================================
// Date/time
// =============================================================================

func nowISO(args []string) string {
	return time.Now().Format(time.RFC3339)
}

func dateFormatVal(args []string) string {
	if len(args) < 2 {
		return safeArg(args, 0)
	}
	// Try common date formats
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"01/02/2006",
	}
	var t time.Time
	parsed := false
	for _, fmt := range formats {
		var err error
		t, err = time.Parse(fmt, args[0])
		if err == nil {
			parsed = true
			break
		}
	}
	if !parsed {
		return args[0]
	}
	// Convert strftime-like format to Go format (basic)
	goFmt := args[1]
	goFmt = strings.ReplaceAll(goFmt, "%Y", "2006")
	goFmt = strings.ReplaceAll(goFmt, "%m", "01")
	goFmt = strings.ReplaceAll(goFmt, "%d", "02")
	goFmt = strings.ReplaceAll(goFmt, "%H", "15")
	goFmt = strings.ReplaceAll(goFmt, "%M", "04")
	goFmt = strings.ReplaceAll(goFmt, "%S", "05")
	return t.Format(goFmt)
}

func dateDiffDays(args []string) string {
	if len(args) < 2 {
		return "0"
	}
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"}
	var t1, t2 time.Time
	for _, fmt := range formats {
		var err error
		t1, err = time.Parse(fmt, args[0])
		if err == nil {
			break
		}
	}
	for _, fmt := range formats {
		var err error
		t2, err = time.Parse(fmt, args[1])
		if err == nil {
			break
		}
	}
	diff := t2.Sub(t1)
	days := int(diff.Hours() / 24)
	return strconv.Itoa(days)
}

// =============================================================================
// Hashing
// =============================================================================

func md5Hash(args []string) string {
	if len(args) == 0 {
		return ""
	}
	h := md5.Sum([]byte(args[0]))
	return fmt.Sprintf("%x", h)
}

func sha256Hash(args []string) string {
	if len(args) == 0 {
		return ""
	}
	h := sha256.Sum256([]byte(args[0]))
	return fmt.Sprintf("%x", h)
}

// =============================================================================
// List
// =============================================================================

func listGet(args []string) string {
	if len(args) < 2 {
		return ""
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(args[0]), &arr); err != nil {
		return ""
	}
	idx, err := strconv.Atoi(args[1])
	if err != nil {
		return ""
	}
	if idx < 0 || idx >= len(arr) {
		return ""
	}
	switch v := arr[idx].(type) {
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func listLength(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(args[0]), &arr); err != nil {
		return "0"
	}
	return strconv.Itoa(len(arr))
}

func listJoin(args []string) string {
	if len(args) < 2 {
		return ""
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(args[0]), &arr); err != nil {
		return ""
	}
	sep := args[1]
	parts := make([]string, 0, len(arr))
	for _, v := range arr {
		switch s := v.(type) {
		case string:
			parts = append(parts, s)
		default:
			b, _ := json.Marshal(s)
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, sep)
}

func listContains(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(args[0]), &arr); err != nil {
		return "false"
	}
	target := args[1]
	for _, v := range arr {
		switch s := v.(type) {
		case string:
			if s == target {
				return "true"
			}
		default:
			b, _ := json.Marshal(s)
			if string(b) == target {
				return "true"
			}
		}
	}
	return "false"
}

func trimTurns(args []string) string {
	if len(args) == 0 {
		return ""
	}
	conversationJSON := args[0]

	// Parse max_turns from args[1], default 10
	maxTurns := 10
	if len(args) >= 2 {
		if n, err := strconv.Atoi(strings.TrimSpace(args[1])); err == nil && n > 0 {
			maxTurns = n
		}
	}

	// Parse the JSON array
	var turns []interface{}
	if err := json.Unmarshal([]byte(conversationJSON), &turns); err != nil {
		// Not valid JSON — return as-is
		return conversationJSON
	}

	// Keep the last maxTurns items
	if len(turns) > maxTurns {
		turns = turns[len(turns)-maxTurns:]
	}

	result, err := json.Marshal(turns)
	if err != nil {
		return conversationJSON
	}
	return string(result)
}

// lenVal is a polymorphic length function.
// - JSON array  → number of elements
// - JSON object → number of keys
// - string      → number of characters
func lenVal(args []string) string {
	if len(args) == 0 {
		return "0"
	}
	s := args[0]
	// Try array
	var arr []interface{}
	if json.Unmarshal([]byte(s), &arr) == nil {
		return strconv.Itoa(len(arr))
	}
	// Try object
	var obj map[string]interface{}
	if json.Unmarshal([]byte(s), &obj) == nil {
		return strconv.Itoa(len(obj))
	}
	// Fall back to string character count
	return strconv.Itoa(len([]rune(s)))
}

// =============================================================================
// Helpers
// =============================================================================

func safeArg(args []string, idx int) string {
	if idx < len(args) {
		return args[idx]
	}
	return ""
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

func formatNum(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
