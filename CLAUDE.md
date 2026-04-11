# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

`spl-go` is the Go implementation of SPL 2.0 (Semantic Prompt Language) — a declarative, SQL-inspired language for LLM-powered agentic workflows. The binary is built as `spl` and aliased as `spl-go` in the shell.

**Python-first discipline:** Features are developed and stabilized in the Python runtime (`digital-duck/SPL20`) before being ported here. Divergences between the two runtimes are bugs in the Go port.

## Build & Run

```bash
# Build
go build -o spl-go .

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/executor/...

# Run a specific test
go test ./internal/executor/ -run TestParallelExecution

# Format
go fmt ./...

# Lint (if golangci-lint is installed)
golangci-lint run
```

## Architecture

The pipeline is: source `.spl` file → **Lexer** → tokens → **Parser** → AST → **Executor** → LLM calls via **Adapter**.

### Core packages (`internal/`)

| Package | Role |
|---|---|
| `lexer/` | Hand-written tokenizer. All SPL keywords are `TokenType` constants. |
| `parser/` | Recursive-descent parser; produces `ast.Program` (list of `ast.Stmt`). |
| `ast/` | AST node types (`PromptStatement`, `WorkflowStatement`, etc.). |
| `executor/` | Runtime engine. Manages variable state (`@vars`), LLM call dispatch, control flow, EXCEPTION handlers. Also contains `Planner` (static token-cost estimation) and `Parallel` (concurrent WORKFLOW step execution). |
| `adapter/` | Pluggable LLM backends implementing the `Adapter` interface. Factory `New(name, cfg)` in `adapter.go`. |
| `stdlib/` | Built-in functions callable from SPL (`upper`, `json_extract`, `now`, etc.). All are `func([]string) string`. |
| `functions/` | User-defined function registry (SPL `CREATE FUNCTION`). |
| `storage/` | SQLite-backed persistence: `kv_store` (persistent `@var` memory) and `prompt_cache`. DB at `~/.spl/memory.db`. |
| `rag/` | ChromaDB + Ollama embedding integration for Doc-RAG and Code-RAG. |
| `text2spl/` | Compiles natural-language descriptions into SPL source via an LLM. |
| `tokencount/` | Token estimation used by the Planner. |
| `config/` | Loads/writes `~/.spl/config.yaml`. |

### CLI (`cmd/`)

Each file maps to a Cobra subcommand (`run`, `validate`, `explain`, `text2spl`, `config`, `memory`, `doc-rag`, `code-rag`, `adapters`, `version`).

### Adapter interface

Every LLM backend must implement four methods (defined in `internal/adapter/adapter.go`):

```go
Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error)
CountTokens(text, model string) int
ListModels() []string
Name() string
```

LLM calls are **blocking, not streaming** — by design for Momagrid's task-queue model.

### Error handling

Runtime errors that SPL programs can catch use `*SPLError` (in `internal/executor/executor.go`). Named constants exist for each error type (`ErrHallucination`, `ErrRefusal`, `ErrContextLength`, etc.).

## Key Config

`~/.spl/config.yaml` — auto-created on first run. Controls default adapter, model, token/cost budgets, and per-adapter settings.

## Optional Services

RAG features require external services:

```bash
ollama serve                                 # local LLM inference + embeddings
docker run -p 8000:8000 chromadb/chroma      # vector store for RAG
ollama pull nomic-embed-text                 # default embedding model
```
