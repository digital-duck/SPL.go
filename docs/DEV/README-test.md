# Testing Guide — spl-go (SPL 2.0 Go Runtime)

---

## Overview

The test suite covers three layers without requiring a live LLM.
All tests use the built-in **echo adapter**, which returns the assembled prompt
as its output — making tests fast, deterministic, and network-free.

| Package | File | What it tests |
|---|---|---|
| `internal/lexer` | `lexer_test.go` | Tokenisation of SPL source |
| `internal/parser` | `parser_test.go` | AST construction from token stream |
| `internal/executor` | `executor_test.go` | End-to-end execution + benchmarks |

---

## Prerequisites

```bash
# Go 1.22+
go version

# Build the binary (optional — only needed for CLI comparison tests)
cd ~/projects/digital-duck/SPL20.go
go build -o spl-go .
./spl-go version
# spl-go — SPL 2.0 Go runtime v0.1.0
```

---

## Run All Unit Tests

```bash
cd ~/projects/digital-duck/SPL20.go

# All three packages at once
go test ./internal/lexer/ ./internal/parser/ ./internal/executor/

# With verbose output (shows each test name)
go test -v ./internal/lexer/ ./internal/parser/ ./internal/executor/

# Short form — all packages in the module
go test ./...
```

Expected output:

```
ok  github.com/digital-duck/spl20go/internal/lexer     0.001s
ok  github.com/digital-duck/spl20go/internal/parser    0.001s
ok  github.com/digital-duck/spl20go/internal/executor  0.005s
```

---

## Run a Specific Package

```bash
go test -v ./internal/lexer/
go test -v ./internal/parser/
go test -v ./internal/executor/
```

## Run a Single Test

```bash
go test -v -run TestSimplePrompt        ./internal/parser/
go test -v -run TestExecuteWorkflow     ./internal/executor/
go test -v -run TestKeywords            ./internal/lexer/
```

---

## Benchmarks (spl-go baseline)

Benchmarks live in `internal/executor/executor_test.go` and measure the Go
runtime with the echo adapter — no LLM latency, pure orchestration overhead.

```bash
# Run benchmarks (5 seconds per benchmark)
go test -bench=. -benchtime=5s ./internal/executor/

# Run benchmarks and include unit tests
go test -bench=. -benchtime=5s -v ./internal/executor/
```

Example output:

```
BenchmarkSimplePromptEcho-8    500000    2100 ns/op
BenchmarkWorkflowEcho-8        200000    6800 ns/op
```

---

## spl vs spl-go Side-by-Side Comparison

The alias `spl` points to the Python implementation; `spl-go` is the Go binary.
Use this pattern to compare correctness and performance on the same `.spl` file.

### Setup alias (if not already set)

```bash
# In ~/.bashrc or ~/.zshrc
alias spl='python -m spl'          # adjust to your Python install
export PATH=$PATH:~/projects/digital-duck/SPL20.go   # for spl-go binary
```

### Correctness check — echo adapter (no LLM)

```bash
# Both should produce equivalent output
spl    run your_file.spl --adapter echo
spl-go run your_file.spl --adapter echo
```

### Performance benchmark — echo adapter

```bash
time spl    run your_file.spl --adapter echo
time spl-go run your_file.spl --adapter echo
```

### Real LLM comparison — Ollama

```bash
# Ensure Ollama is running with a model pulled
ollama pull llama3.2

time spl    run your_file.spl --adapter ollama -m llama3.2
time spl-go run your_file.spl --adapter ollama -m llama3.2
```

Metrics to compare:

| Metric | What to look for |
|---|---|
| Wall time | Go cold-start should be significantly faster |
| Token counts | Should match between implementations |
| Output quality | Responses should be semantically equivalent |
| Memory usage | `/usr/bin/time -v` to capture peak RSS |

---

## Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./internal/lexer/ ./internal/parser/ ./internal/executor/

# View in terminal
go tool cover -func=coverage.out

# View as HTML in browser
go tool cover -html=coverage.out
```

---

## Adding New Tests

### Unit test (no LLM)

Place `_test.go` files alongside the package they test.
Use the echo adapter for any execution test:

```go
exec := executor.New(adapter.NewEchoAdapter())
results, err := exec.ExecuteProgram(ctx, prog, params)
```

### Test with a real adapter (integration)

Guard with a build tag or environment variable so CI doesn't require Ollama:

```go
func TestWithOllama(t *testing.T) {
    if os.Getenv("INTEGRATION") == "" {
        t.Skip("set INTEGRATION=1 to run")
    }
    // ... test against real Ollama
}
```

Run with:

```bash
INTEGRATION=1 go test -v -run TestWithOllama ./internal/executor/
```

---

## SPL Syntax Quick Reference for Test Fixtures

```spl
-- Simple prompt (top-level)
PROMPT name
SELECT
    system_role('You are helpful'),
    context.topic
GENERATE llm('Explain the topic')

-- Prompt with model and budget
PROMPT name USING MODEL 'llama3.2' WITH BUDGET 512 TOKENS
SELECT system_role('sys')
GENERATE llm('task')

-- Workflow with variable assignment
WORKFLOW name
INPUT: @param1, @param2
OUTPUT: @result
DO
    @result := 'literal value'
END

-- Workflow with LLM call (GENERATE INTO — not PROMPT inside DO)
WORKFLOW name
INPUT: @topic
OUTPUT: @result
DO
    GENERATE llm(@topic) INTO @result
END
```

> **Note:** Inside a `WORKFLOW DO` block, LLM calls use `GENERATE llm(...) INTO @var`.
> Top-level `PROMPT` statements are not valid inside a workflow body.
> Workflow status is `"no_commit"` unless a `COMMIT` statement is reached.
