# SPL20.go — Port Roadmap

_Last updated: 2026-03-28 (evening)_

This document tracks what has been ported from the Python implementation
(`digital-duck/SPL20`) to the Go runtime (`digital-duck/SPL20.go`), what is
planned next, and what is intentionally deferred.

Mark items `[ ]` → `[x]` as you complete them.

---

## Completed (as of 2026-03-28)

### Core pipeline
- [x] Lexer — all token types (100% parity with Python)
- [x] Parser — all 18 statement types
- [x] AST nodes
- [x] Executor core — statement dispatch, workflow state machine
- [x] Exception handling — all 8 exception types
- [x] Budget enforcement — `max_llm_calls`, `max_total_tokens`
- [x] Stdlib — 47 functions including `trim_turns`
- [x] Missing-feature warnings — stderr warnings instead of silent failures

### Storage
- [x] Memory store — SQLite-backed `~/.spl/memory.db` (`kv_store` + `prompt_cache`)
- [x] `STORE @var IN memory.key` in executor
- [x] `MemoryGet` expression (`memory.key`) in executor

### Adapters
- [x] `echo` — deterministic testing adapter
- [x] `ollama` — local Ollama server
- [x] `momagrid` — distributed LAN inference grid
- [x] `anthropic` — Anthropic Claude API
- [x] `claude_cli` — Claude Code CLI (subscription billing, $0/call)
- [x] `openrouter` — OpenRouter (100+ models, 3-pass JSON parser, reasoning model fallback)
- [x] `openai` — OpenAI API (gpt-4o, o1, o3 family)
- [x] `deepseek` — DeepSeek API, reasoning model fallback
- [x] `qwen` — Alibaba Cloud DashScope (qwen-plus, qwen-max, qwen2.5-* family)

### RAG
- [x] ChromaDB REST client + Ollama embedding foundation (`internal/rag/chroma.go`)
- [x] Doc-RAG — paragraph-chunked document indexing (`internal/rag/docrag.go`)
- [x] Code-RAG — (description, SPL source) pair indexing (`internal/rag/coderag.go`)

### text2spl compiler
- [x] `internal/text2spl/compiler.go` — LLM + CodeRAG examples + parse/validate + retry loop

### Config
- [x] Per-adapter defaults (base URL, model, timeout)
- [x] Safety caps (`max_llm_calls`, `max_total_tokens`)
- [x] `text2spl`, `code_rag`, `doc_rag` config sections

### Runtime internals
- [x] `tokencount` — model-aware chars/token ratio (claude=3.5, gpt=4.0, gemini=3.8, llama=3.5)
- [x] `analyzer` — semantic analysis: duplicate names, exception type validation, temperature/budget bounds
- [x] `functions` — FunctionRegistry: builtins (summarize, list_*, file I/O) + user-defined `CREATE FUNCTION` + `CALL`
- [x] Goroutine pool — parallel execution of independent workflow steps via `--workers N`
- [x] `WorkflowState` mutex — thread-safe variable reads/writes for concurrent steps

### CLI
- [x] `spl-go run` — execute SPL programs
- [x] `spl-go run --workers N` — parallel step execution via goroutine pool
- [x] `spl-go validate` — syntax check + semantic analysis warnings
- [x] `spl-go explain` — structure summary with analysis results
- [x] `spl-go adapters` — list adapters + env vars
- [x] `spl-go version`
- [x] `spl-go init` — create `~/.spl/` and default `config.yaml`
- [x] `spl-go config show/get/set/path`
- [x] `spl-go memory list/get/set/delete` (SQLite)
- [x] `spl-go cache list/clear` (SQLite prompt_cache)
- [x] `spl-go doc-rag add/query/count`
- [x] `spl-go code-rag import/add/query/count/export/parse-log`
- [x] `spl-go text2spl` — with `--mode`, `-o`, `--execute`, `--no-code-rag`, `--no-validate`

---

## Pending — mark what to port next

Mark `[ ]` → `[x]` and I will implement.

### Cloud provider adapters — implement when Momagrid WAN deployment is funded

All three exist in the Python implementation (`spl/adapters/`). Port to Go when
Momagrid is deployed to cloud infrastructure (AWS, GCP, Azure).

- [ ] `google` — Google Generative AI SDK / Gemini (`GOOGLE_API_KEY`)
- [ ] `azure_openai` — Azure OpenAI (`AZURE_OPENAI_ENDPOINT` + `AZURE_OPENAI_API_KEY`)
- [ ] `bedrock` — AWS Bedrock Converse API (AWS credentials: env / profile / IAM role; supports Claude, Llama, Nova, cross-region inference profiles)

---

## Benchmark target (2026-03-30)

5-node Momagrid grid (4× GTX 1080 Ti + 1× RTX 4060 8 GB):

- [ ] Run full 40-recipe cookbook with `spl-go` + `--adapter momagrid`
- [ ] Compare wall-clock vs Python `spl` baseline (1197.6s single-node)
- [ ] Record pass rate — any failure = Go port bug to fix

---

## Deferred — Python-only, do not port

| Feature | Reason |
|---|---|
| `vertex` adapter | GCP Vertex AI OAuth2 — superseded by `google` adapter via API key |
| Streamlit UI (`spl ui`) | No Go equivalent planned |
| `asyncio` concurrency | **Done** — replaced with goroutine pool (`--workers N`) |
