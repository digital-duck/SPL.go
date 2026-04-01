# SPL20.go — Semantic Prompt Language Go Runtime

This project is a Go implementation of **SPL 2.0** (Semantic Prompt Language), a declarative, SQL-inspired language for LLM-powered agentic workflows. It is designed for production use and single-binary deployment, particularly alongside Momagrid nodes.

## Project Overview

- **Core Purpose:** Execute SPL programs with minimal overhead and zero external dependencies (single binary).
- **Primary Tech:** Go 1.22+, Cobra (CLI), SQLite (local storage), YAML (config).
- **Architecture:** 
  - **Lexer/Parser:** Hand-written recursive descent parser for the SPL grammar.
  - **Executor:** Runtime engine that manages variable state, LLM calls, and workflow control flow.
  - **Adapters:** Plug-gable backends for LLM inference (Ollama, Anthropic, Momagrid, Claude CLI, etc.).
  - **RAG:** Integrated document and code retrieval using ChromaDB and Ollama embeddings.
  - **Memory:** SQLite-backed persistent key-value store and prompt cache.

## Building and Running

### Build
```bash
go build -o spl-go .
```

### Run SPL Programs
```bash
# Basic run
spl-go run workflow.spl

# Run with parameters
spl-go run workflow.spl topic="generative ai" limit=5

# Specify adapter and model
spl-go run workflow.spl -a anthropic -m claude-3-5-sonnet
```

### Testing
```bash
go test ./...
```

### Linting
```bash
# Standard Go formatting
go fmt ./...
# Recommended: golangci-lint
golangci-lint run
```

## Development Conventions

### Python-First Discipline
- **Stability First:** Features are implemented and stabilized in the Python runtime (`digital-duck/SPL20`) before being ported to Go.
- **Parity is Priority:** Any recipe that passes in the Python runtime MUST pass in the Go runtime. Divergences are considered bugs.

### Coding Style
- **Surgical Changes:** Keep edits focused on the specific task. Avoid unrelated refactors.
- **Internal Package:** Core logic (lexer, parser, executor, adapters) resides in `internal/` to keep the public API surface small.
- **Adapter Interface:** New LLM backends must implement the `Adapter` interface in `internal/adapter/adapter.go`.
- **Error Handling:** Use custom `SPLError` types in the executor to support SPL `EXCEPTION` handlers.

### RAG Integration
- Uses **ChromaDB** as the vector store.
- Uses **Ollama** (`nomic-embed-text`) for local embeddings by default.
- Configuration is managed in `~/.spl/config.yaml`.

## Key Files & Directories

- `main.go`: Application entry point.
- `cmd/`: CLI command definitions (Cobra).
- `internal/lexer/`: SPL lexical analysis.
- `internal/parser/`: SPL syntax analysis and AST generation.
- `internal/executor/`: The core runtime engine.
- `internal/adapter/`: LLM backend implementations.
- `internal/stdlib/`: Standard library functions (string, math, json, etc.).
- `internal/storage/`: SQLite-based persistence layer.
- `docs/DESIGN.md`: Deep dive into architectural decisions and roadmap.
