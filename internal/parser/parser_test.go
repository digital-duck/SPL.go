package parser

import (
	"testing"

	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/lexer"
)

// parse is a helper that lexes + parses source, failing the test on any error.
func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	l := lexer.New(src)
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	p := New(tokens)
	prog, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return prog
}

func mustFail(t *testing.T, src string) {
	t.Helper()
	l := lexer.New(src)
	tokens, err := l.Tokenize()
	if err != nil {
		return // lex error is also acceptable
	}
	p := New(tokens)
	_, err = p.Parse()
	if err == nil {
		t.Errorf("expected parse error for %q, got nil", src)
	}
}

// ── PROMPT Statement ──────────────────────────────────────────────────────────

func TestSimplePrompt(t *testing.T) {
	src := `PROMPT greet
SELECT system_role('You are helpful')
GENERATE llm('Say hello')`
	prog := parse(t, src)

	if len(prog.Statements) != 1 {
		t.Fatalf("want 1 statement, got %d", len(prog.Statements))
	}
	stmt, ok := prog.Statements[0].(*ast.PromptStatement)
	if !ok {
		t.Fatalf("want *PromptStatement, got %T", prog.Statements[0])
	}
	if stmt.Name != "greet" {
		t.Errorf("want name 'greet', got %q", stmt.Name)
	}
	if stmt.GenerateClause == nil {
		t.Error("want GenerateClause, got nil")
	}
	if stmt.GenerateClause.FunctionName != "llm" {
		t.Errorf("want generate fn 'llm', got %q", stmt.GenerateClause.FunctionName)
	}
}

func TestPromptWithModel(t *testing.T) {
	src := `PROMPT analyse USING MODEL 'llama3'
SELECT system_role('You are an analyst')
GENERATE llm('Explain SPL')`
	prog := parse(t, src)

	stmt := prog.Statements[0].(*ast.PromptStatement)
	if stmt.Model != "llama3" {
		t.Errorf("want model 'llama3', got %q", stmt.Model)
	}
}

func TestPromptWithBudget(t *testing.T) {
	src := `PROMPT summary WITH BUDGET 512 TOKENS
SELECT system_role('Summarise this')
GENERATE llm('Give a brief summary')`
	prog := parse(t, src)

	stmt := prog.Statements[0].(*ast.PromptStatement)
	if stmt.Budget != 512 {
		t.Errorf("want budget 512, got %d", stmt.Budget)
	}
}

func TestPromptSelectSystemRole(t *testing.T) {
	src := `PROMPT p SELECT system_role('Act as expert') GENERATE llm('task')`
	prog := parse(t, src)
	stmt := prog.Statements[0].(*ast.PromptStatement)

	if len(stmt.SelectItems) != 1 {
		t.Fatalf("want 1 select item, got %d", len(stmt.SelectItems))
	}
	_, ok := stmt.SelectItems[0].Expression.(*ast.SystemRoleCall)
	if !ok {
		t.Errorf("want SystemRoleCall, got %T", stmt.SelectItems[0].Expression)
	}
}

func TestPromptSelectContextRef(t *testing.T) {
	src := `PROMPT p SELECT context.topic GENERATE llm('discuss')`
	prog := parse(t, src)
	stmt := prog.Statements[0].(*ast.PromptStatement)

	ref, ok := stmt.SelectItems[0].Expression.(*ast.ContextRef)
	if !ok {
		t.Fatalf("want *ContextRef, got %T", stmt.SelectItems[0].Expression)
	}
	if ref.FieldName != "topic" {
		t.Errorf("want field 'topic', got %q", ref.FieldName)
	}
}

func TestMultipleSelectItems(t *testing.T) {
	src := `PROMPT p
SELECT
    system_role('You are helpful'),
    context.background
GENERATE llm('summarise')`
	prog := parse(t, src)
	stmt := prog.Statements[0].(*ast.PromptStatement)

	if len(stmt.SelectItems) != 2 {
		t.Errorf("want 2 select items, got %d", len(stmt.SelectItems))
	}
}

// ── WORKFLOW Statement ────────────────────────────────────────────────────────

func TestSimpleWorkflow(t *testing.T) {
	src := `WORKFLOW translate
INPUT: @topic
OUTPUT: @result
DO
    @result := 'done'
END`
	prog := parse(t, src)

	if len(prog.Statements) != 1 {
		t.Fatalf("want 1 statement, got %d", len(prog.Statements))
	}
	wf, ok := prog.Statements[0].(*ast.WorkflowStatement)
	if !ok {
		t.Fatalf("want *WorkflowStatement, got %T", prog.Statements[0])
	}
	if wf.Name != "translate" {
		t.Errorf("want name 'translate', got %q", wf.Name)
	}
	if len(wf.Inputs) != 1 || wf.Inputs[0].Name != "topic" {
		t.Errorf("want input 'topic', got %v", wf.Inputs)
	}
}

// ── Multiple Statements ───────────────────────────────────────────────────────

func TestMultipleStatements(t *testing.T) {
	src := `PROMPT first SELECT system_role('sys') GENERATE llm('task1')
PROMPT second SELECT system_role('sys') GENERATE llm('task2')`
	prog := parse(t, src)

	if len(prog.Statements) != 2 {
		t.Errorf("want 2 statements, got %d", len(prog.Statements))
	}
}

func TestSemicolonSeparator(t *testing.T) {
	src := `PROMPT first SELECT system_role('sys') GENERATE llm('t1');
PROMPT second SELECT system_role('sys') GENERATE llm('t2');`
	prog := parse(t, src)

	if len(prog.Statements) != 2 {
		t.Errorf("want 2 statements, got %d", len(prog.Statements))
	}
}

// ── Comments Are Ignored ──────────────────────────────────────────────────────

func TestCommentIgnored(t *testing.T) {
	src := `-- Recipe 01
PROMPT hello SELECT system_role('help') GENERATE llm('hi')`
	prog := parse(t, src)
	if len(prog.Statements) != 1 {
		t.Errorf("want 1 statement, got %d", len(prog.Statements))
	}
}

// ── Error Cases ───────────────────────────────────────────────────────────────

func TestMissingPromptName(t *testing.T) {
	mustFail(t, `PROMPT SELECT system_role('x') GENERATE llm('y')`)
}

func TestMissingSelectKeyword(t *testing.T) {
	mustFail(t, `PROMPT p system_role('x') GENERATE llm('y')`)
}

func TestMissingWorkflowEnd(t *testing.T) {
	mustFail(t, `WORKFLOW w DO @x := 'hello'`)
}
