package lexer

import (
	"testing"
)

// tokenTypes extracts just the token types from a slice.
func tokenTypes(tokens []Token) []TokenType {
	types := make([]TokenType, len(tokens))
	for i, t := range tokens {
		types[i] = t.Type
	}
	return types
}

func mustTokenize(t *testing.T, src string) []Token {
	t.Helper()
	l := New(src)
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}
	return tokens
}

// ── Keywords ──────────────────────────────────────────────────────────────────

func TestKeywords(t *testing.T) {
	cases := []struct {
		src      string
		wantType TokenType
	}{
		{"PROMPT", PROMPT},
		{"prompt", PROMPT},
		{"SELECT", SELECT},
		{"GENERATE", GENERATE},
		{"WORKFLOW", WORKFLOW},
		{"WITH", WITH},
		{"USING", USING},
		{"MODEL", MODEL},
		{"BUDGET", BUDGET},
		{"TOKENS", TOKENS},
		{"END", END},
		{"DO", DO},
		{"INPUT", INPUT},
		{"OUTPUT", OUTPUT},
		{"EVALUATE", EVALUATE},
		{"WHEN", WHEN},
		{"THEN", THEN},
		{"ELSE", ELSE},
		{"otherwise", ELSE}, // backward-compat alias
		// SPL 3.0
		{"IMPORT", IMPORT},
		{"import", IMPORT},
		{"PARALLEL", PARALLEL},
		{"parallel", PARALLEL},
		{"IMAGE", IMAGE},
		{"image", IMAGE},
		{"AUDIO", AUDIO},
		{"audio", AUDIO},
		{"VIDEO", VIDEO},
		{"video", VIDEO},
	}
	for _, tc := range cases {
		tokens := mustTokenize(t, tc.src)
		if len(tokens) < 1 || tokens[0].Type != tc.wantType {
			t.Errorf("src=%q: want token %d, got %d", tc.src, tc.wantType, tokens[0].Type)
		}
	}
}

// ── Identifiers ───────────────────────────────────────────────────────────────

func TestIdentifier(t *testing.T) {
	tokens := mustTokenize(t, "my_workflow")
	if tokens[0].Type != IDENTIFIER || tokens[0].Value != "my_workflow" {
		t.Errorf("expected IDENTIFIER 'my_workflow', got %v", tokens[0])
	}
}

// ── String Literals ───────────────────────────────────────────────────────────

func TestStringSingleQuote(t *testing.T) {
	tokens := mustTokenize(t, "'hello world'")
	if tokens[0].Type != STRING || tokens[0].Value != "hello world" {
		t.Errorf("expected STRING 'hello world', got %v", tokens[0])
	}
}

func TestStringDoubleQuote(t *testing.T) {
	tokens := mustTokenize(t, `"hello world"`)
	if tokens[0].Type != STRING || tokens[0].Value != "hello world" {
		t.Errorf("expected STRING 'hello world', got %v", tokens[0])
	}
}

func TestStringEscapeSequences(t *testing.T) {
	tokens := mustTokenize(t, `"line1\nline2"`)
	if tokens[0].Type != STRING || tokens[0].Value != "line1\nline2" {
		t.Errorf("unexpected value: %q", tokens[0].Value)
	}
}

func TestDollarDollarString(t *testing.T) {
	tokens := mustTokenize(t, "$$multi\nline\nbody$$")
	// Should produce: DOLLAR_DOLLAR, STRING, EOF
	if len(tokens) < 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != DOLLAR_DOLLAR {
		t.Errorf("expected DOLLAR_DOLLAR, got %v", tokens[0])
	}
	if tokens[1].Type != STRING || tokens[1].Value != "multi\nline\nbody" {
		t.Errorf("expected STRING body, got %v", tokens[1])
	}
}

func TestFString(t *testing.T) {
	tokens := mustTokenize(t, `f"hello {name}"`)
	if tokens[0].Type != FSTRING {
		t.Errorf("expected FSTRING, got %v", tokens[0])
	}
}

func TestUnterminatedString(t *testing.T) {
	l := New(`"unterminated`)
	_, err := l.Tokenize()
	if err == nil {
		t.Error("expected error for unterminated string, got nil")
	}
}

func TestUnterminatedDollarDollar(t *testing.T) {
	l := New("$$no closing")
	_, err := l.Tokenize()
	if err == nil {
		t.Error("expected error for unterminated $$ string, got nil")
	}
}

// ── Numbers ───────────────────────────────────────────────────────────────────

func TestInteger(t *testing.T) {
	tokens := mustTokenize(t, "42")
	if tokens[0].Type != INTEGER || tokens[0].Value != "42" {
		t.Errorf("expected INTEGER '42', got %v", tokens[0])
	}
}

func TestFloat(t *testing.T) {
	tokens := mustTokenize(t, "3.14")
	if tokens[0].Type != FLOAT || tokens[0].Value != "3.14" {
		t.Errorf("expected FLOAT '3.14', got %v", tokens[0])
	}
}

// ── Operators ─────────────────────────────────────────────────────────────────

func TestOperators(t *testing.T) {
	cases := []struct {
		src  string
		want TokenType
	}{
		{":=", ASSIGN},
		{":", COLON},
		{"!=", NEQ},
		{">=", GTE},
		{"<=", LTE},
		{">", GT},
		{"<", LT},
		{"=", EQ},
		{"||", PIPE_PIPE},
		{"|", PIPE},
		{"@", AT},
	}
	for _, tc := range cases {
		tokens := mustTokenize(t, tc.src)
		if tokens[0].Type != tc.want {
			t.Errorf("src=%q: want %d, got %d", tc.src, tc.want, tokens[0].Type)
		}
	}
}

// ── Comments ──────────────────────────────────────────────────────────────────

func TestLineComment(t *testing.T) {
	tokens := mustTokenize(t, "-- this is a comment\nPROMPT")
	if tokens[0].Type != PROMPT {
		t.Errorf("expected PROMPT after comment, got %v", tokens[0])
	}
}

// ── Line / Column Tracking ────────────────────────────────────────────────────

func TestLineTracking(t *testing.T) {
	tokens := mustTokenize(t, "PROMPT\nhello")
	if tokens[1].Line != 2 {
		t.Errorf("expected token on line 2, got line %d", tokens[1].Line)
	}
}

// ── Unexpected Character ──────────────────────────────────────────────────────

func TestUnexpectedChar(t *testing.T) {
	l := New("PROMPT hello !")
	_, err := l.Tokenize()
	if err == nil {
		t.Error("expected error for unexpected '!', got nil")
	}
}

// ── Full PROMPT Snippet ───────────────────────────────────────────────────────

func TestPromptSnippetTokenStream(t *testing.T) {
	src := `PROMPT greet USING MODEL 'llama3' SELECT system_role('You are helpful') GENERATE llm('Say hi')`
	tokens := mustTokenize(t, src)

	types := tokenTypes(tokens)
	want := []TokenType{
		PROMPT, IDENTIFIER, // PROMPT greet
		USING, MODEL, STRING, // USING MODEL 'llama3'
		SELECT, IDENTIFIER, LPAREN, STRING, RPAREN, // SELECT system_role('You are helpful')
		GENERATE, IDENTIFIER, LPAREN, STRING, RPAREN, // GENERATE llm('Say hi')
		EOF,
	}
	if len(types) != len(want) {
		t.Fatalf("token count: want %d got %d\ntypes: %v", len(want), len(types), types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("token[%d]: want %d got %d", i, want[i], types[i])
		}
	}
}
