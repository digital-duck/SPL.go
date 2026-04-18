// run_all.go — SPL 2.0 Cookbook batch runner (Go port of run_all.py).
//
// Reads cookbook_catalog.json from the SPL20 cookbook directory and runs
// active recipes via the spl-go binary.  All .spl files are read from the
// SPL20 repo — this file only replaces the Python launcher.
//
// Usage (from SPL20/cookbook):
//
//	go run /path/to/SPL20.go/cookbook/run_all.go
//	go run /path/to/SPL20.go/cookbook/run_all.go --adapter ollama
//	go run /path/to/SPL20.go/cookbook/run_all.go --adapter momagrid --workers 5
//	go run /path/to/SPL20.go/cookbook/run_all.go --ids 04,08,10-13
//	go run /path/to/SPL20.go/cookbook/run_all.go --list
//	go run /path/to/SPL20.go/cookbook/run_all.go --catalog
//
// Or build once and use directly:
//
//	go build -o ~/bin/spl-run /path/to/SPL20.go/cookbook/run_all.go
//	spl-run --adapter momagrid --workers 10

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Catalog types ─────────────────────────────────────────────────────────────

type Recipe struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Args           []string `json:"args"`
	Dir            string   `json:"dir"`
	Log            string   `json:"log"`
	IsActive       bool     `json:"is_active"`
	ApprovalStatus string   `json:"approval_status"`
	Category       string   `json:"category"`
}

type Catalog struct {
	Recipes []Recipe `json:"recipes"`
}

type Result struct {
	ID      string
	Name    string
	OK      bool
	Elapsed float64
}

// ── Catalog helpers ────────────────────────────────────────────────────────────

func loadCatalog(cookbookDir string) ([]Recipe, error) {
	path := filepath.Join(cookbookDir, "cookbook_catalog.json")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open catalog: %w", err)
	}
	defer f.Close()
	var cat Catalog
	if err := json.NewDecoder(f).Decode(&cat); err != nil {
		return nil, fmt.Errorf("cannot parse catalog: %w", err)
	}
	return cat.Recipes, nil
}

func statusMarker(r Recipe) string {
	if r.IsActive {
		return "✅"
	}
	switch r.ApprovalStatus {
	case "new":
		return "🆕"
	case "wip":
		return "🔧"
	case "disabled":
		return "⏸ "
	case "rejected":
		return "❌"
	default:
		return "  "
	}
}

func applyFilters(recipes []Recipe, category, status string) []Recipe {
	var out []Recipe
	for _, r := range recipes {
		if category != "" && r.Category != category {
			continue
		}
		if status != "" && r.ApprovalStatus != status {
			continue
		}
		out = append(out, r)
	}
	return out
}

// parseIDFilter parses "04,08,10-13" into a set of zero-padded ID strings.
func parseIDFilter(ids string) map[string]bool {
	set := map[string]bool{}
	for _, part := range strings.Split(ids, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			sides := strings.SplitN(part, "-", 2)
			lo, _ := strconv.Atoi(strings.TrimSpace(sides[0]))
			hi, _ := strconv.Atoi(strings.TrimSpace(sides[1]))
			for n := lo; n <= hi; n++ {
				set[fmt.Sprintf("%02d", n)] = true
			}
		} else {
			n, err := strconv.Atoi(part)
			if err == nil {
				set[fmt.Sprintf("%02d", n)] = true
			} else {
				set[part] = true
			}
		}
	}
	return set
}

// applyOverrides splices --adapter / -m into the args slice.
func applyOverrides(args []string, adapter, model string) []string {
	if len(args) == 0 || args[0] != "spl" {
		return args
	}
	result := make([]string, len(args))
	copy(result, args)

	if adapter != "" {
		if i := indexOf(result, "--adapter"); i >= 0 {
			result[i+1] = adapter
		} else {
			// insert after the .spl file argument
			for i, a := range result {
				if strings.HasSuffix(a, ".spl") {
					result = insert(result, i+1, "--adapter", adapter)
					break
				}
			}
		}
	}
	if model != "" {
		if i := indexOf(result, "--model"); i >= 0 {
			result[i+1] = model
		} else {
			result = append(result, "--model", model)
		}
	}
	return result
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

func insert(slice []string, idx int, vals ...string) []string {
	out := make([]string, 0, len(slice)+len(vals))
	out = append(out, slice[:idx]...)
	out = append(out, vals...)
	out = append(out, slice[idx:]...)
	return out
}

// ── Display helpers ────────────────────────────────────────────────────────────

func printList(recipes []Recipe, category, status string) {
	filtered := applyFilters(recipes, category, status)
	label := ""
	if category != "" || status != "" {
		label = fmt.Sprintf(" (category=%q status=%q)", category, status)
	}
	fmt.Printf("SPL 2.0 Cookbook — %d recipes%s\n", len(filtered), label)
	for _, r := range filtered {
		fmt.Printf("  %-4s  %s  %-28s  %-12s  %-14s  %s\n",
			r.ID, statusMarker(r), r.Name, r.ApprovalStatus, r.Category, r.Description)
	}
}

func printCatalog(recipes []Recipe, category, status string) {
	filtered := applyFilters(recipes, category, status)
	now := time.Now().Format("2006-01-02 15:04:05")

	counts := map[string]int{}
	catCounts := map[string]int{}
	for _, r := range recipes {
		if r.IsActive {
			counts["active"]++
		}
		counts[r.ApprovalStatus]++
		catCounts[r.Category]++
	}

	fmt.Printf("=== SPL 2.0 Cookbook Catalog — %s ===\n", now)
	if category != "" || status != "" {
		fmt.Printf("    Filter: category=%q  status=%q  → %d/%d recipes\n\n", category, status, len(filtered), len(recipes))
	} else {
		fmt.Printf("    Total: %d recipes  |  %d active  |  %d new  |  %d wip  |  %d disabled\n\n",
			len(recipes), counts["active"], counts["new"], counts["wip"], counts["disabled"])
	}

	fmt.Printf("%-4s  %-2s  %-28s  %-14s  %-12s  %s\n", "ID", "", "Name", "Category", "Status", "Description")
	fmt.Println(strings.Repeat("-", 110))
	for _, r := range filtered {
		fmt.Printf("%-4s  %s  %-28s  %-14s  %-12s  %s\n",
			r.ID, statusMarker(r), r.Name, r.Category, r.ApprovalStatus, r.Description)
	}

	fmt.Println()
	fmt.Println("Markers: ✅ active  🆕 new  🔧 wip  ⏸  disabled  ❌ rejected")
	fmt.Println()
	fmt.Println("Run active recipes:           spl-run")
	fmt.Println("Run specific recipe:          spl-run --ids 05,11")
	fmt.Println("Override adapter/model:       spl-run --adapter momagrid --model llama3.2")
	fmt.Println("Filter catalog by category:   spl-run --catalog --category agentic")
	fmt.Println("Filter catalog by status:     spl-run --catalog --status new")

	cats := make([]string, 0, len(catCounts))
	for c := range catCounts {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	parts := make([]string, 0, len(cats))
	for _, c := range cats {
		parts = append(parts, fmt.Sprintf("%s(%d)", c, catCounts[c]))
	}
	fmt.Printf("\nCategories: %s\n", strings.Join(parts, "  "))
}

func printSummary(results []Result, startAll time.Time) {
	total := len(results)
	passed := 0
	for _, r := range results {
		if r.OK {
			passed++
		}
	}
	elapsed := time.Since(startAll).Seconds()
	fmt.Printf("\n=== Summary: %d/%d Success  (total %.1fs) ===\n\n", passed, total, elapsed)
	fmt.Printf("%-4s  %-28s  %-8s  %8s\n", "ID", "Recipe", "Status", "Elapsed")
	fmt.Println(strings.Repeat("-", 56))
	for _, r := range results {
		s := "OK"
		if !r.OK {
			s = "FAILED"
		}
		fmt.Printf("%-4s  %-28s  %-8s  %7.1fs\n", r.ID, r.Name, s, r.Elapsed)
	}
	fmt.Println()
}

// ── Recipe execution ───────────────────────────────────────────────────────────

// runSequential streams recipe output with fence-aware formatting and logs to file.
func runSequential(cmdArgs []string, logPath, cwd string) (bool, float64) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "     | ERROR mkdir: %v\n", err)
	}
	start := time.Now()

	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "     | ERROR creating log: %v\n", err)
		return false, 0
	}
	defer logFile.Close()

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec
	cmd.Dir = cwd

	pr, pw, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "     | ERROR pipe: %v\n", err)
		return false, 0
	}
	cmd.Stdout = io.MultiWriter(logFile, pw)
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "     | ERROR starting: %v\n", err)
		pw.Close()
		pr.Close()
		return false, 0
	}
	pw.Close()

	// Read output line-by-line with fence-aware formatting
	buf := make([]byte, 0, 4096)
	inFence := false
	tmp := make([]byte, 1)
	for {
		n, err := pr.Read(tmp)
		if n > 0 {
			if tmp[0] == '\n' {
				line := string(buf)
				buf = buf[:0]
				if inFence {
					fmt.Println(line)
					if strings.TrimSpace(line) == "```" {
						inFence = false
					}
				} else {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "```") && trimmed != "```" {
						fmt.Println(line)
						inFence = true
					} else {
						fmt.Printf("     | %s\n", line)
					}
				}
			} else {
				buf = append(buf, tmp[0])
			}
		}
		if err != nil {
			break
		}
	}
	// flush any remaining bytes without newline
	if len(buf) > 0 {
		fmt.Printf("     | %s\n", string(buf))
	}

	ok := cmd.Wait() == nil
	return ok, time.Since(start).Seconds()
}

// runParallel runs a recipe in background, writing output directly to log file.
func runParallel(r Recipe, cmdArgs []string, logPath, cwd string) Result {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] ERROR mkdir: %v\n", r.ID, err)
	}
	start := time.Now()
	fmt.Printf("[%s] %s  →  started\n", r.ID, r.Name)

	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] ERROR creating log: %v\n", r.ID, err)
		return Result{ID: r.ID, Name: r.Name, OK: false}
	}
	defer logFile.Close()

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec
	cmd.Dir = cwd
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	ok := cmd.Run() == nil
	elapsed := time.Since(start).Seconds()
	status := "SUCCESS"
	if !ok {
		status = "FAILED"
	}
	fmt.Printf("[%s] %s  →  %s  (%.1fs)  log: %s\n", r.ID, r.Name, status, elapsed, filepath.Base(logPath))
	return Result{ID: r.ID, Name: r.Name, OK: ok, Elapsed: elapsed}
}

// ── Main ───────────────────────────────────────────────────────────────────────

func main() {
	adapter  := flag.String("adapter", "", "Override LLM adapter for all recipes (e.g. ollama, momagrid)")
	model    := flag.String("model", "", "Override model for all recipes")
	ids      := flag.String("ids", "", "Comma-separated recipe IDs or ranges (e.g. '04,08,10-13')")
	workers  := flag.Int("workers", 0, "Max parallel workers (default: number of active recipes)")
	category := flag.String("category", "", "Only run recipes in this category")
	status   := flag.String("status", "", "Only run recipes with this approval_status")
	listFlag := flag.Bool("list", false, "Print brief recipe list and exit")
	catalog  := flag.Bool("catalog", false, "Print full catalog table and exit")
	splRepo  := flag.String("spl-repo", "", "Path to SPL20 cookbook dir (default: auto-detect from $SPL20_REPO or sibling dir)")
	flag.Parse()

	cookbookDir := resolveCookbookDir(*splRepo)

	recipes, err := loadCatalog(cookbookDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if *catalog {
		printCatalog(recipes, *category, *status)
		return
	}
	if *listFlag {
		printList(recipes, *category, *status)
		return
	}

	useParallel := *adapter == "momagrid"
	idFilter := map[string]bool{}
	if *ids != "" {
		idFilter = parseIDFilter(*ids)
	}

	startAll := time.Now()
	fmt.Printf("=== SPL 2.0 Cookbook Batch Run — %s ===\n", startAll.Format("2006-01-02 15:04:05"))
	var overrides []string
	if *adapter != "" {
		overrides = append(overrides, "adapter="+*adapter)
	}
	if *model != "" {
		overrides = append(overrides, "model="+*model)
	}
	if len(overrides) > 0 {
		fmt.Printf("    Overrides : %s\n", strings.Join(overrides, ", "))
	}
	if useParallel {
		fmt.Println("    Mode      : parallel (momagrid — recipes submitted concurrently)")
	}
	fmt.Println()

	ts := time.Now().Format("20060102_150405")

	// Build work list
	type work struct {
		r       Recipe
		cmdArgs []string
		logPath string
	}
	var active []work
	for _, r := range recipes {
		if len(idFilter) > 0 {
			if !idFilter[r.ID] {
				continue
			}
		} else {
			if *category != "" && r.Category != *category {
				continue
			}
			if *status != "" && r.ApprovalStatus != *status {
				continue
			}
			if !r.IsActive {
				fmt.Printf("[%s] %s  (skipping — %s)\n", r.ID, r.Name, strings.ToUpper(r.ApprovalStatus))
				continue
			}
		}
		cmdArgs := applyOverrides(r.Args, *adapter, *model)
		logPath := filepath.Join(cookbookDir, r.Dir, fmt.Sprintf("%s_%s.md", r.Log, ts))
		active = append(active, work{r, cmdArgs, logPath})
	}

	if len(active) == 0 {
		fmt.Println("No recipes to run.")
		return
	}

	var results []Result

	if useParallel {
		n := *workers
		if n == 0 {
			n = len(active)
		}
		fmt.Printf("Submitting %d recipe(s) with %d parallel worker(s)...\n\n", len(active), n)

		sem := make(chan struct{}, n)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, w := range active {
			w := w
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				res := runParallel(w.r, w.cmdArgs, w.logPath, cookbookDir)
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}()
		}
		wg.Wait()
		sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	} else {
		for _, w := range active {
			fmt.Printf("[%s] %s\n", w.r.ID, w.r.Name)
			fmt.Printf("     cmd : %s\n", strings.Join(w.cmdArgs, " "))
			fmt.Printf("     log : %s\n", w.logPath)
			ok, elapsed := runSequential(w.cmdArgs, w.logPath, cookbookDir)
			status := "SUCCESS"
			if !ok {
				status = "FAILED"
			}
			fmt.Printf("     result: %s  (%.1fs)\n\n", status, elapsed)
			results = append(results, Result{ID: w.r.ID, Name: w.r.Name, OK: ok, Elapsed: elapsed})
		}
	}

	printSummary(results, startAll)
}

// resolveCookbookDir finds the SPL20/cookbook directory.
// Priority: --spl-repo flag → $SPL20_REPO env → sibling "SPL20" dir → current dir.
func resolveCookbookDir(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("SPL20_REPO"); env != "" {
		return filepath.Join(env, "cookbook")
	}
	// Try sibling of this binary's parent
	exe, _ := os.Executable()
	candidates := []string{
		filepath.Join(filepath.Dir(exe), "..", "SPL20", "cookbook"),
		filepath.Join(os.Getenv("HOME"), "projects", "digital-duck", "SPL20", "cookbook"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "cookbook_catalog.json")); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	// Fall back to current directory
	cwd, _ := os.Getwd()
	return cwd
}
