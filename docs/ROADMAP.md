# SPL.go — Port Roadmap

_Last updated: 2026-04-11_

This document tracks what has been ported from the Python implementations
(SPL v1.0, v2.0, v3.0) to the Go runtime (`digital-duck/SPL.go`), what is
planned next, and what is intentionally deferred.

---

## Completed

### Core pipeline
- [x] Lexer — all token types (SPL v1.0 + v2.0 + v3.0 keywords)
- [x] Parser — all statement types including SPL 3.0 constructs
- [x] AST nodes
- [x] Executor core — statement dispatch, workflow state machine
- [x] Exception handling — all 8 exception types
- [x] Budget enforcement — `max_llm_calls`, `max_total_tokens`
- [x] Stdlib — 47 functions including `trim_turns`
- [x] Missing-feature warnings — stderr warnings instead of silent failures
- [x] **Lightweight Planner** — pre-execution resource estimates (LLM calls, tokens, cost) via `--plan`
- [x] **Idiomatic Error Aggregation** — `errors.Join` for multi-errors in parallel workflows

### SPL 3.0 language features (ported 2026-04-11)
- [x] **`IMPORT 'file.spl'`** — load external SPL files; merges FUNCTION/PROCEDURE/WORKFLOW definitions; transitive imports supported; paths resolve relative to calling file
- [x] **`CALL PARALLEL ... END`** — concurrent workflow/procedure fan-out via goroutines; snapshot-per-branch variable isolation; metrics aggregated back to parent
- [x] **`IMAGE`, `AUDIO`, `VIDEO` type keywords** — valid `INPUT`/`OUTPUT` parameter types; values held as strings (file path or base64 data URL)
- [x] **`CALL` dispatches `WORKFLOW` definitions** — `CALL workflow_name(args) INTO @var` now resolves registered workflows (previously only procedures/builtins)
- [x] **`Executor.Workflows` registry** — workflows registered by name at load time, enabling cross-file dispatch

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
- [x] **Dependency-Aware Parallelization** — grouping of independent workflow steps for goroutine pool
- [x] `WorkflowState` mutex — thread-safe variable reads/writes for concurrent steps

### CLI
- [x] `spl-go run` — execute SPL programs
- [x] `spl-go run --workers N` — parallel step execution via goroutine pool
- [x] `spl-go run --plan` — show resource estimates before execution
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

## Pending

### Cloud provider adapters — implement when Momagrid WAN deployment is funded

All three exist in the Python v2.0 runtime. Port to Go when Momagrid is deployed
to cloud infrastructure (AWS, GCP, Azure).

- [ ] `google` — Google Generative AI / Gemini (`GOOGLE_API_KEY`)
- [ ] `azure_openai` — Azure OpenAI (`AZURE_OPENAI_ENDPOINT` + `AZURE_OPENAI_API_KEY`)
- [ ] `bedrock` — AWS Bedrock Converse API (AWS credentials: env / profile / IAM role; supports Claude, Llama, Nova)

### SPL 3.0 multimodal — implement when SPL30 stabilizes

- [ ] `internal/codec/image.go` — PNG/JPEG/WebP → base64 data URL (Go stdlib `image/*`)
- [ ] `internal/codec/audio.go` — WAV/MP3 → base64 bytes
- [ ] `internal/codec/video.go` — MP4 → frame slice at configurable FPS (ffmpeg subprocess)
- [ ] `MultimodalAdapter` interface — extends `Adapter` with `GenerateMultimodal(ctx, []ContentBlock, ...)`
- [ ] `ContentBlock` type — `{Type, Text, Source, MediaType, Data}`
- [ ] Upgrade `anthropic`, `openai` to implement `MultimodalAdapter`
- [ ] `internal/adapter/liquid.go` — Liquid inference (native multimodal)
- [ ] `internal/adapter/snap.go` — Ubuntu AI snap stub (Ubuntu 26.04 not yet GA)
- [ ] Executor integration — auto-encode IMAGE/AUDIO/VIDEO vars to `ContentBlock` on `GENERATE`

### SPL 3.0 workflow registry + hub peering — implement when Momagrid hub API is stable

- [ ] In-process workflow registry (`internal/registry/`) — named workflows resolved at CALL time
- [ ] Hub-backed registry — `GET /workflows/{name}` for remote dispatch
- [ ] Hub-to-Hub peering — design work needed

### Benchmark target (4-node Momagrid grid)

- [ ] Run full 40-recipe cookbook with `spl-go` + `--adapter momagrid`
- [ ] Compare wall-clock vs Python `spl` baseline (1197.6s single-node, 383.7s 3-node)
- [ ] Record pass rate — any failure = Go port bug to fix

---

## Deferred — Python-only, do not port

| Feature | Reason |
|---|---|
| `vertex` adapter | GCP Vertex AI OAuth2 — superseded by `google` adapter via API key |
| Streamlit UI (`spl ui`) | No Go equivalent planned |
| `asyncio` concurrency | **Done** — replaced with goroutine pool (`--workers N`) and `CALL PARALLEL` |
