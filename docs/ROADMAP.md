# SPL20.go — Port Roadmap

_Last updated: 2026-03-28_

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

### CLI
- [x] `gspl run` — execute SPL programs
- [x] `gspl validate` — syntax check
- [x] `gspl explain` — structure summary
- [x] `gspl adapters` — list adapters + env vars
- [x] `gspl version`
- [x] `gspl config show/get/set/path`
- [x] `gspl memory list/get/set/delete` (SQLite)
- [x] `gspl doc-rag add/query/count`
- [x] `gspl code-rag import/add/query/count`
- [x] `gspl text2spl` — with `--mode`, `-o`, `--execute`, `--no-code-rag`, `--no-validate`

---

## Pending — mark what to port next

Mark `[ ]` → `[x]` and I will implement.

### Group 1 — Adapters (low effort, high value)

`openai`, `deepseek`, `qwen` all use the OpenAI-compatible `/chat/completions` protocol —
trivial to add once `openrouter` is done.

- [x] `openai` — OpenAI API (`OPENAI_API_KEY`)
- [x] `deepseek` — DeepSeek API, OpenAI-compatible (`DEEPSEEK_API_KEY`)
- [x] `qwen` — Alibaba Cloud Qwen, OpenAI-compatible (`QWEN_API_KEY`)
- [ ] `google` — Google Generative AI REST (different schema, medium effort)
- [ ] `azure_openai` — OpenAI-compatible + deployment name in URL + API version header
- [ ] `bedrock` — AWS SigV4 signing required (high effort — see Deferred)
- [ ] `vertex` — GCP OAuth2 required (high effort — see Deferred)

### Group 2 — Runtime internals

- [x] `token_counter` — model-aware chars/token ratio (claude=3.5, gpt=4.0, gemini=3.8, llama=3.5); improves cost accuracy across all adapters
- [x] `analyzer` — semantic analysis: variable scope, undefined references, exception type validation; improves `gspl validate`
- [x] `functions` — `FunctionRegistry`: user-defined SPL functions (`CREATE FUNCTION`) + `CALL` statement execution
- [x] `explain` — richer `gspl explain` output: execution plan, CTE fan-outs, estimated LLM calls (needs `analyzer` first)
- [x] `optimizer` — execution plan builder (prerequisite for parallel step execution)
- [x] `ir` — intermediate representation (required by `optimizer`)

### Group 3 — CLI commands (low effort, storage already in place)

- [x] `gspl init` — create `~/.spl/` and write default `config.yaml`
- [x] `gspl cache list` — list cached prompts from `prompt_cache` SQLite table
- [x] `gspl cache clear` — clear all cached prompts
- [x] `gspl code-rag export -o file.jsonl` — export all pairs as JSONL for fine-tuning
- [x] `gspl code-rag parse-log <run.md>` — extract (description, SPL) pairs from `run_all.py` logs

---

## Suggested implementation order

```
1. openai + deepseek + qwen adapters      ~1 hr    — unlocks most cloud models
2. token_counter                          ~30 min  — better cost accuracy everywhere
3. gspl init + cache list/clear           ~30 min  — CLI completeness
4. code-rag export + parse-log            ~30 min  — useful for fine-tuning
5. analyzer                               ~2-3 hrs — richer gspl validate
6. functions (FunctionRegistry + CALL)    ~2-3 hrs — CALL statement support
7. google + azure_openai adapters         ~2 hrs   — after openai is done
8. explain (richer output)                ~1 hr    — needs analyzer
9. optimizer + IR                         future   — parallel execution
```

---

## Benchmark target (2026-03-30)

5-node Momagrid grid (4× GTX 1080 Ti + 1× RTX 4060 8 GB):

- [ ] Run full 40-recipe cookbook with `gspl` + `--adapter momagrid`
- [ ] Compare wall-clock vs Python `spl` baseline (1197.6s single-node)
- [ ] Record pass rate — any failure = Go port bug to fix

---

## Deferred — Python-only, do not port

| Feature | Reason |
|---|---|
| `bedrock` adapter | AWS SigV4 credential signing — complex, low priority |
| `vertex` adapter | GCP OAuth2 — complex, low priority |
| Streamlit UI (`spl ui`) | No Go equivalent planned |
| `asyncio` concurrency | Go uses sync executor; goroutine pool is a future option |
