package executor

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newEchoExecutor() *Executor {
	return New(adapter.NewEchoAdapter())
}

func runSrc(t *testing.T, src string, params map[string]string) []interface{} {
	t.Helper()
	l := lexer.New(src)
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	p := parser.New(tokens)
	prog, err := p.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exec := newEchoExecutor()
	results, err := exec.ExecuteProgram(context.Background(), prog, params)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return results
}

func firstSPLResult(t *testing.T, results []interface{}) *SPLResult {
	t.Helper()
	if len(results) == 0 {
		t.Fatal("no results")
	}
	r, ok := results[0].(*SPLResult)
	if !ok {
		t.Fatalf("want *SPLResult, got %T", results[0])
	}
	return r
}

func firstWorkflowResult(t *testing.T, results []interface{}) *WorkflowResult {
	t.Helper()
	if len(results) == 0 {
		t.Fatal("no results")
	}
	r, ok := results[0].(*WorkflowResult)
	if !ok {
		t.Fatalf("want *WorkflowResult, got %T", results[0])
	}
	return r
}

// ── PROMPT Execution ──────────────────────────────────────────────────────────

func TestExecuteSimplePrompt(t *testing.T) {
	src := `PROMPT greet
SELECT system_role('You are helpful')
GENERATE llm('Say hello')`

	results := runSrc(t, src, nil)
	r := firstSPLResult(t, results)

	// Echo adapter echoes the assembled prompt back as content
	if r.Content == "" {
		t.Error("expected non-empty content from echo adapter")
	}
	if r.Model != "echo" {
		t.Errorf("expected model 'echo', got %q", r.Model)
	}
}

func TestExecutePromptContainsTask(t *testing.T) {
	src := `PROMPT p SELECT system_role('sys') GENERATE llm('explain quantum computing')`
	results := runSrc(t, src, nil)
	r := firstSPLResult(t, results)

	if !strings.Contains(r.Content, "explain quantum computing") {
		t.Errorf("expected content to contain task text, got:\n%s", r.Content)
	}
}

func TestExecutePromptWithContextParam(t *testing.T) {
	src := `PROMPT p SELECT context.topic GENERATE llm('discuss the topic')`
	results := runSrc(t, src, map[string]string{"topic": "distributed inference"})
	r := firstSPLResult(t, results)

	if !strings.Contains(r.Content, "distributed inference") {
		t.Errorf("expected content to contain param value, got:\n%s", r.Content)
	}
}

func TestExecutePromptWithModel(t *testing.T) {
	// Model field is parsed and stored; echo adapter ignores it but we verify no error
	src := `PROMPT p USING MODEL 'llama3'
SELECT system_role('sys')
GENERATE llm('task')`
	results := runSrc(t, src, nil)
	if len(results) != 1 {
		t.Errorf("want 1 result, got %d", len(results))
	}
}

func TestExecuteMultiplePrompts(t *testing.T) {
	src := `PROMPT first SELECT system_role('sys') GENERATE llm('task1')
PROMPT second SELECT system_role('sys') GENERATE llm('task2')`
	results := runSrc(t, src, nil)
	if len(results) != 2 {
		t.Errorf("want 2 results, got %d", len(results))
	}
}

// ── Token Counting (Echo Adapter) ─────────────────────────────────────────────

func TestEchoAdapterTokenCount(t *testing.T) {
	src := `PROMPT p SELECT system_role('sys') GENERATE llm('hello world')`
	results := runSrc(t, src, nil)
	r := firstSPLResult(t, results)

	// Echo adapter: tokens = len(prompt)/4
	if r.InputTokens <= 0 {
		t.Errorf("expected positive input token count, got %d", r.InputTokens)
	}
	if r.CostUSD != 0.0 {
		t.Errorf("echo adapter should have zero cost, got %f", r.CostUSD)
	}
}

// ── WORKFLOW Execution ────────────────────────────────────────────────────────

func TestExecuteWorkflowAssignment(t *testing.T) {
	// Workflow without COMMIT returns "no_commit" status; variables are still in Output.
	src := `WORKFLOW greet
INPUT: @name
OUTPUT: @result
DO
    @result := 'hello'
END`
	results := runSrc(t, src, map[string]string{"name": "world"})
	r := firstWorkflowResult(t, results)

	if r.Status != "no_commit" {
		t.Errorf("expected status 'no_commit', got %q", r.Status)
	}
	if r.Output["result"] != "hello" {
		t.Errorf("expected output result='hello', got %q", r.Output["result"])
	}
}

func TestExecuteWorkflowWithGenerateInto(t *testing.T) {
	// Inside a workflow body, LLM calls use: GENERATE llm(@var) INTO @target
	src := `WORKFLOW analyse
INPUT: @topic
OUTPUT: @analysis
DO
    GENERATE llm(@topic) INTO @analysis
END`
	results := runSrc(t, src, map[string]string{"topic": "Go binary deployment"})
	r := firstWorkflowResult(t, results)

	if r.TotalLLMCalls != 1 {
		t.Errorf("expected 1 LLM call, got %d", r.TotalLLMCalls)
	}
	if r.Output["analysis"] == "" {
		t.Error("expected non-empty analysis output")
	}
}

func TestWorkflowOutputContainsInputParam(t *testing.T) {
	src := `WORKFLOW echo_topic
INPUT: @topic
OUTPUT: @out
DO
    GENERATE llm(@topic) INTO @out
END`
	results := runSrc(t, src, map[string]string{"topic": "momagrid snap"})
	r := firstWorkflowResult(t, results)

	if !strings.Contains(r.Output["out"], "momagrid snap") {
		t.Errorf("expected output to contain param value, got: %q", r.Output["out"])
	}
}

// ── Adapter Interface ─────────────────────────────────────────────────────────

func TestEchoAdapterInterface(t *testing.T) {
	a := adapter.NewEchoAdapter()
	if a.Name() != "echo" {
		t.Errorf("expected name 'echo', got %q", a.Name())
	}
	models := a.ListModels()
	if len(models) != 1 || models[0] != "echo" {
		t.Errorf("expected ['echo'], got %v", models)
	}
	count := a.CountTokens("hello world", "echo")
	if count <= 0 {
		t.Errorf("expected positive token count, got %d", count)
	}
}

func TestEchoAdapterGenerate(t *testing.T) {
	a := adapter.NewEchoAdapter()
	res, err := a.Generate(context.Background(), "test prompt", "echo", 100, 0.7, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "test prompt" {
		t.Errorf("echo should return prompt as content, got %q", res.Content)
	}
	if res.LatencyMs <= 0 {
		t.Errorf("expected positive latency, got %f", res.LatencyMs)
	}
}

// ── spl vs spl-go Benchmark Harness ──────────────────────────────────────────
//
// Run with: go test -bench=. -benchtime=5s ./internal/executor/
//
// Use these as the baseline when comparing against the Python 'spl' command:
//   time spl run testdata/bench_simple.spl --adapter echo
//   time spl-go run testdata/bench_simple.spl --adapter echo

func BenchmarkSimplePromptEcho(b *testing.B) {
	src := `PROMPT bench SELECT system_role('sys') GENERATE llm('bench task')`
	l := lexer.New(src)
	tokens, _ := l.Tokenize()
	p := parser.New(tokens)
	prog, _ := p.Parse()
	exec := newEchoExecutor()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteProgram(ctx, prog, nil)
	}
}

func BenchmarkWorkflowEcho(b *testing.B) {
	src := `WORKFLOW bench
INPUT: @topic
OUTPUT: @result
DO
    GENERATE llm(@topic) INTO @result
END`
	l := lexer.New(src)
	tokens, _ := l.Tokenize()
	p := parser.New(tokens)
	prog, _ := p.Parse()
	exec := newEchoExecutor()
	ctx := context.Background()
	params := map[string]string{"topic": "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteProgram(ctx, prog, params)
	}
}

// ── MapLiteral, StorageSubscript, StorageAssignStatement ─────────────────────

func TestMapLiteralEval(t *testing.T) {
	src := `WORKFLOW test_map
INPUT:  @unused TEXT
OUTPUT: @result TEXT
DO
    @m := {'lang': 'SPL', 'version': '2.0'}
    @result := @m
    COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"unused": ""})
	r := firstWorkflowResult(t, results)
	// result should be a JSON object string
	if !strings.Contains(r.CommittedValue, "SPL") {
		t.Errorf("expected JSON map with 'SPL', got: %s", r.CommittedValue)
	}
	if !strings.Contains(r.CommittedValue, "lang") {
		t.Errorf("expected key 'lang' in map, got: %s", r.CommittedValue)
	}
}

func TestEmptyMapLiteral(t *testing.T) {
	src := `WORKFLOW test_empty_map
INPUT:  @unused TEXT
OUTPUT: @result TEXT
DO
    @m := {}
    @result := @m
    COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"unused": ""})
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "{}" {
		t.Errorf("expected '{}', got: %s", r.CommittedValue)
	}
}

func TestStorageSubscriptReadMap(t *testing.T) {
	src := `WORKFLOW test_subscript
INPUT:  @unused TEXT
OUTPUT: @result TEXT
DO
    @m := {'key': 'hello', 'other': 'world'}
    @result := @m['key']
    COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"unused": ""})
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "hello" {
		t.Errorf("expected 'hello', got: %s", r.CommittedValue)
	}
}

func TestStorageSubscriptReadList(t *testing.T) {
	src := `WORKFLOW test_list_subscript
INPUT:  @unused TEXT
OUTPUT: @result TEXT
DO
    @items := ['alpha', 'beta', 'gamma']
    @result := @items['1']
    COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"unused": ""})
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "beta" {
		t.Errorf("expected 'beta', got: %s", r.CommittedValue)
	}
}

func TestStorageAssignStatement(t *testing.T) {
	src := `WORKFLOW test_storage_assign
INPUT:  @unused TEXT
OUTPUT: @result TEXT
DO
    @m := {}
    @m['name'] := 'SPL'
    @m['year'] := '2026'
    @result := @m['name']
    COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"unused": ""})
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "SPL" {
		t.Errorf("expected 'SPL', got: %s", r.CommittedValue)
	}
}

func TestStorageAssignUpdatesExistingMap(t *testing.T) {
	src := `WORKFLOW test_storage_update
INPUT:  @unused TEXT
OUTPUT: @result TEXT
DO
    @m := {'key': 'original'}
    @m['key'] := 'updated'
    @result := @m['key']
    COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"unused": ""})
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "updated" {
		t.Errorf("expected 'updated', got: %s", r.CommittedValue)
	}
}

// ── SPL 3.0: CALL PARALLEL ────────────────────────────────────────────────────

func TestCallParallelTwoProcedures(t *testing.T) {
	// Two procedures run in parallel; both results should appear in parent state.
	src := `
PROCEDURE greet(name text) RETURNS text
DO
  GENERATE g(name) INTO @out
  COMMIT @out
END

PROCEDURE shout(name text) RETURNS text
DO
  GENERATE s(name) INTO @out
  COMMIT @out
END

WORKFLOW parallel_test
  INPUT:  @name text DEFAULT 'world'
  OUTPUT: @result text
DO
  CALL PARALLEL
    greet(@name) INTO @hello,
    shout(@name) INTO @loud
  END
  @result := @hello
  COMMIT @result
END`
	results := runSrc(t, src, map[string]string{"name": "world"})
	r := firstWorkflowResult(t, results)
	// With echo adapter, GENERATE returns the prompt text, not empty string.
	if r.CommittedValue == "" {
		t.Error("expected non-empty committed value from CALL PARALLEL workflow")
	}
}

func TestCallParallelResultsIsolated(t *testing.T) {
	// Each branch must only write to its own INTO @var, not bleed into sibling state.
	src := `
PROCEDURE left_branch() RETURNS text
DO
  @private := 'left-only'
  COMMIT @private
END

PROCEDURE right_branch() RETURNS text
DO
  @private := 'right-only'
  COMMIT @private
END

WORKFLOW isolation_test
  OUTPUT: @result text
DO
  CALL PARALLEL
    left_branch() INTO @left,
    right_branch() INTO @right
  END
  @result := @left
  COMMIT @result
END`
	results := runSrc(t, src, nil)
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "left-only" {
		t.Errorf("want committed value 'left-only', got %q", r.CommittedValue)
	}
}

// ── SPL 3.0: IMPORT ───────────────────────────────────────────────────────────

func TestImportLoadsDefinitions(t *testing.T) {
	// Write a temporary helper .spl file, then IMPORT it from the main program.
	dir := t.TempDir()
	helperPath := dir + "/helpers.spl"
	helperSrc := `
PROCEDURE echo_name(name text) RETURNS text
DO
  COMMIT name
END`
	if err := os.WriteFile(helperPath, []byte(helperSrc), 0600); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	mainSrc := `
IMPORT 'helpers.spl'

WORKFLOW main
  INPUT:  @name text DEFAULT 'alice'
  OUTPUT: @result text
DO
  CALL echo_name(@name) INTO @result
  COMMIT @result
END`

	l := lexer.New(mainSrc)
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	p := parser.New(tokens)
	prog, err := p.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	exec := newEchoExecutor()
	exec.SourceDir = dir // so IMPORT resolves relative to the temp dir
	results, err := exec.ExecuteProgram(context.Background(), prog, map[string]string{"name": "alice"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	r := firstWorkflowResult(t, results)
	if r.CommittedValue != "alice" {
		t.Errorf("want committed value 'alice', got %q", r.CommittedValue)
	}
}
