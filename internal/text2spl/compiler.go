// Package text2spl converts natural language descriptions to SPL 2.0 source code.
package text2spl

import (
	"context"
	"fmt"
	"strings"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
	"github.com/digital-duck/spl20go/internal/rag"
)

// spl2Syntax is the embedded SPL 2.0 syntax reference injected into the system prompt.
const spl2Syntax = `
SPL 2.0 (Semantic Prompt Language) — Syntax Reference

## PROMPT statement
Defines a reusable LLM prompt with optional budget, model, context, and generate clause.

  PROMPT <name> [WITH BUDGET <n> TOKENS] [USING MODEL '<model>']
  SELECT
    system_role('<description>'),
    context.<field> [AS <alias>],
    memory.get('<key>') [AS <alias>],
    rag.query('<text>', top_k=<n>) [AS <alias>],
    <expr> [AS <alias>]
  [WHERE <condition>]
  [ORDER BY <field> ASC|DESC]
  GENERATE <function>(<args>)
  [WITH OUTPUT BUDGET <n> TOKENS] [TEMPERATURE <f>] [FORMAT JSON] [SCHEMA '...']
  [STORE RESULT IN memory.<key>]

## WORKFLOW statement
Defines an agentic workflow with input/output params, body, and exception handling.

  WORKFLOW <name>
  INPUT (@<param> <TYPE> [DEFAULT <value>], ...)
  OUTPUT (@<var>, ...)
  [SECURITY (owner='...', classification='...')]
  [ACCOUNTING (cost_center='...', budget_usd=<n>)]
  DO
    <statements>
  [EXCEPTION
    WHEN <ExceptionType> THEN
      <statements>
    WHEN OTHERS THEN
      <statements>
  ]
  END

## PROCEDURE statement
  PROCEDURE <name>(@<param> <TYPE> [DEFAULT <value>], ...) RETURNS <type>
  DO
    <statements>
  [EXCEPTION WHEN <type> THEN <statements>]
  END

## Workflow body statements

  -- Assignment
  @<var> := <expr>

  -- Generate LLM result into a variable
  GENERATE <function>(<args>) [WITH OUTPUT BUDGET <n>] [TEMPERATURE <f>] [MODEL '<m>'] INTO @<var>

  -- Select with CTE into variable(s)
  [WITH <name> AS (
    SELECT ... GENERATE <func>(<args>)
  )]
  SELECT <expr> [AS <alias>], ...
  [FROM <source>]
  INTO @<var1> [, @<var2>, ...]

  -- CALL a procedure or stdlib function
  CALL <procedure>(<args>) [INTO @<var>]

  -- EVALUATE (semantic branching)
  EVALUATE @<var>
  WHEN '<semantic-condition>' THEN
    <statements>
  WHEN > <n> THEN
    <statements>
  ELSE
    <statements>
  END

  -- WHILE loop
  WHILE '<semantic-condition>' [MAX ITERATIONS <n>] DO
    <statements>
  END

  -- DO block with exception handling
  DO
    <statements>
  EXCEPTION
    WHEN <ExceptionType> THEN <statements>
  END

  -- COMMIT (end workflow with a value)
  COMMIT @<var> [WITH status='<s>', quality_score=<n>]

  -- RETRY (in exception handler)
  RETRY [WITH model='<m>', temperature=<f>] [LIMIT <n>]

  -- RAISE an exception
  RAISE <ExceptionType> '<message>'

  -- LOGGING
  LOGGING @<var> [LEVEL DEBUG|INFO|WARN|ERROR] [TO '<file>']

  -- STORE to persistent memory
  STORE @<var> IN memory.<key>

## Built-in exception types
  HallucinationDetected, RefusalToAnswer, ContextLengthExceeded,
  ModelOverloaded, QualityBelowThreshold, MaxIterationsReached,
  BudgetExceeded, NodeUnavailable

## Expression types
  @var          -- variable reference
  'string'      -- string literal
  42            -- integer literal
  3.14          -- float literal
  f'{@var} ...' -- f-string
  a || b        -- string concatenation
  a + b         -- numeric add

## Standard library functions (callable via CALL or GENERATE)
  string_ops: upper, lower, trim, length, contains, replace, split, join
  math_ops: abs, round, min, max
  json_ops: json_parse, json_stringify, json_get
  text_ops: word_count, char_count, truncate, summarize_to_n_words
  list_ops: list_length, list_get, list_append
  time_ops: now, format_date
  llm_utils: trim_turns, extract_code, extract_json
`

// staticExamples are injected when CodeRAG is disabled.
const staticExamples = `
## Example 1: Simple PROMPT
PROMPT summarize_article WITH BUDGET 500 TOKENS
SELECT
  system_role('You are a concise summarizer.'),
  context.article_text AS article
GENERATE summarize(article, 'in 3 bullet points')
WITH OUTPUT BUDGET 300 TOKENS

## Example 2: WORKFLOW with EVALUATE
WORKFLOW quality_check
INPUT (@text TEXT, @threshold INTEGER DEFAULT 7)
OUTPUT (@result)
DO
  GENERATE score_quality(@text) WITH OUTPUT BUDGET 20 TOKENS MODEL 'llama3.2' INTO @score
  EVALUATE @score
  WHEN >= @threshold THEN
    COMMIT @text WITH status='approved'
  ELSE
    RAISE QualityBelowThreshold 'Score too low'
  END
EXCEPTION
  WHEN QualityBelowThreshold THEN
    @result := 'rejected'
    COMMIT @result WITH status='rejected'
END
`

// Compiler converts natural language descriptions to SPL 2.0 source.
type Compiler struct {
	adapter    adapter.Adapter
	codeRAG    *rag.CodeRAGStore // nil = disabled
	maxRetries int
}

// New creates a new Compiler.
func New(adp adapter.Adapter, codeRAG *rag.CodeRAGStore, maxRetries int) *Compiler {
	return &Compiler{
		adapter:    adp,
		codeRAG:    codeRAG,
		maxRetries: maxRetries,
	}
}

// Compile converts a natural language description to SPL 2.0 source.
// mode: "prompt", "workflow", or "auto"
func (c *Compiler) Compile(ctx context.Context, description, mode string) (string, error) {
	examples := staticExamples

	// If CodeRAG is enabled, retrieve relevant examples.
	if c.codeRAG != nil {
		retrieved, err := c.codeRAG.Retrieve(ctx, description, 3)
		if err == nil && len(retrieved) > 0 {
			var sb strings.Builder
			sb.WriteString("\n## Retrieved SPL Examples (most relevant to your request)\n")
			for i, r := range retrieved {
				sb.WriteString(fmt.Sprintf("\n### Example %d: %s\n```spl\n%s\n```\n", i+1, r["description"], r["spl_source"]))
			}
			examples = sb.String()
		}
	}

	modeInstruction := ""
	switch strings.ToLower(mode) {
	case "prompt":
		modeInstruction = "Generate a SPL 2.0 PROMPT statement only (no WORKFLOW)."
	case "workflow":
		modeInstruction = "Generate a SPL 2.0 WORKFLOW statement only (no standalone PROMPT)."
	default: // "auto"
		modeInstruction = "Decide whether to use a PROMPT or WORKFLOW based on the description. Use PROMPT for single-step queries and WORKFLOW for multi-step agentic processes."
	}

	systemPrompt := fmt.Sprintf(`You are an expert SPL 2.0 (Semantic Prompt Language) code generator.

%s

Output only valid SPL 2.0 source code — no prose, no markdown fences, no explanation.
If you include code fences (` + "```" + `) they will be stripped automatically.

%s

%s`, modeInstruction, spl2Syntax, examples)

	userPrompt := fmt.Sprintf("Generate SPL 2.0 code for the following task:\n\n%s", description)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		prompt := userPrompt
		if attempt > 0 && lastErr != nil {
			prompt = fmt.Sprintf("%s\n\nPrevious attempt produced invalid SPL. Error: %v\n\nFix the code and output only valid SPL 2.0:", userPrompt, lastErr)
		}

		result, err := c.adapter.Generate(ctx, prompt, "", 2000, 0.2, systemPrompt)
		if err != nil {
			return "", fmt.Errorf("text2spl: LLM error: %w", err)
		}

		splSource := stripFences(result.Content)

		valid, errMsg := c.Validate(ctx, splSource)
		if valid {
			return splSource, nil
		}
		lastErr = fmt.Errorf("%s", errMsg)
	}

	return "", fmt.Errorf("text2spl: failed to generate valid SPL after %d attempts: %v", c.maxRetries+1, lastErr)
}

// Validate lexes and parses SPL source, returning (true, "") on success.
func (c *Compiler) Validate(_ context.Context, splSource string) (bool, string) {
	tokens, err := lexer.New(splSource).Tokenize()
	if err != nil {
		return false, fmt.Sprintf("lex error: %v", err)
	}
	p := parser.New(tokens)
	if _, err := p.Parse(); err != nil {
		return false, fmt.Sprintf("parse error: %v", err)
	}
	return true, ""
}

// stripFences removes ```spl ... ``` or ``` ... ``` markdown wrapping if present.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	// Remove opening fence
	for _, fence := range []string{"```spl", "```SPL", "```"} {
		if strings.HasPrefix(s, fence) {
			s = s[len(fence):]
			break
		}
	}
	// Remove closing fence
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}
