# SPL.py → SPL.go Sync Plan (2026-07-13)

Reference commit in SPL.py: `4bc6135` (latest as of 2026-07-13)  
Last Go sync point: mid-April 2025  
**Completed:** 2026-07-13 — commit `9962fa1`

## dd-* Dependency Mapping

| dd-* Library | Used for in SPL.py | Go treatment | Done? |
|---|---|---|---|
| `dd-db` | `STORAGE(sqlite/duckdb/postgres, path)` SPL feature | `internal/storage/storage_conn.go` — SQLite wired, DuckDB/Postgres stubs | ✓ |
| `dd-cache` | `DiskCache` prompt_cache in `storage/memory.py` | Already done — `storage/memory.go` uses SQLite | ✓ |
| `dd-embed` | Embeddings for RAG | Go uses Ollama — equivalent, no change needed | ✓ |
| `dd-extract` | PDF text extraction in doc-rag | `cmd/docrag_cmd.go` — calls `pdftotext` (poppler) subprocess with graceful fallback | ✓ |
| `dd-logging` | `setup_logging()` / `get_logger()` in CLI | `log_level` + `log_console` config fields added; Go uses stdlib `log` | ✓ |
| `dd-llm` | Python bridge adapter `dd_llm_bridge.py` | Python-only concept; Go has native adapters — no port needed | n/a |
| `dd-vectordb` | Vector store for RAG | Go uses ChromaDB REST directly — equivalent, no change needed | ✓ |
| `dd-config` | Config management | `storage_dir`, `log_level`, `log_console` added; defaults aligned with Python | ✓ |

## Work Items

### Group A — Core Language (HIGH, in scope)

| # | Item | Files | Status |
|---|---|---|---|
| 1 | **Lexer**: `NONE` token, `TILDE (~)` operator, triple-quoted strings (`"""/'''`), block comments (`/* */`), `#` line comments | `internal/lexer/lexer.go` | ✓ |
| 2 | **Parser**: `INTO NONE` (discard output), `~'cond'` semantic EVALUATE, `IS`/`IS NOT` WHEN sugar, `IN`/`NOT IN` conditions, boolean `WHEN TRUE/FALSE`, `USING MODEL` after `INTO` | `internal/parser/parser.go` | ✓ |
| 3 | **Stdlib**: add `len_val()` polymorphic length (string / JSON array / JSON object) | `internal/stdlib/stdlib.go` | ✓ |
| 4 | **STORAGE param**: SQLite backend fully wired; DuckDB/Postgres stubs with helpful error | `internal/executor/executor.go`, `internal/storage/storage_conn.go` (new) | ✓ |

### Group B — New Adapters (SKIPPED for now)

| # | Item | Status |
|---|---|---|
| 5 | Bedrock adapter | ✗ skip |
| 6 | Gemini CLI adapter | ✗ skip |
| 7 | Vertex AI adapter | ✗ skip |
| 8 | Azure OpenAI adapter | ✗ skip |

### Group C — Config (MEDIUM, in scope except adapter sections)

| # | Item | Files | Status |
|---|---|---|---|
| 9 | Add `storage_dir`, `log_level`, `log_console` fields; align defaults (`max_llm_calls`→25, `max_total_tokens`→100000) | `internal/config/config.go` | ✓ |
| 10 | Add `bedrock`/`vertex`/`azure_openai` config sections | — | ✗ skip |

### Group D — Executor (MEDIUM, in scope)

| # | Item | Files | Status |
|---|---|---|---|
| 11 | New exception types: `ToolFailed`, `FileNotFound`, `UnsupportedFormat`, `CodecError`, `NoAudioTrack`, `InvalidTimestamp` | `internal/executor/executor.go` | ✓ |

### Group E — Lower Priority (in scope)

| # | Item | Files | Status |
|---|---|---|---|
| 12 | IR module — AST→JSON serialization for cross-language interop | `internal/ir/ir.go` (new) | ✓ |
| 13 | Optimizer improvements — `WorkflowStep`/`WorkflowBranch`/`WorkflowPlan` types + `PlanWorkflow`/`PlanProcedure` methods | `internal/executor/planner.go` | ✓ |
| 14 | PDF extraction — `pdftotext` subprocess with graceful fallback | `cmd/docrag_cmd.go` | ✓ |

## Key Python Source References

- Lexer/tokens: `spl/lexer.py`, `spl/tokens.py`
- AST nodes: `spl/ast_nodes.py`
- Parser: `spl/parser.py`
- Executor: `spl/executor.py`
- Stdlib: `spl/stdlib.py`
- Storage conn: `spl/storage/storage_conn.py`
- Memory store: `spl/storage/memory.py`
- Optimizer: `spl/optimizer.py`
- IR: `spl/ir.py`
- Config: `spl/config.py`
- CLI: `spl/cli.py`

## Implementation Notes (shipped 2026-07-13)

### Files modified
| File | Change |
|---|---|
| `internal/lexer/lexer.go` | `NONE` token, `TILDE` operator, triple-quoted strings (`"""/'''`), `#` line + `/* */` block comments |
| `internal/parser/parser.go` | `INTO NONE`, `~'cond'` semantic EVALUATE, `IS`/`IS NOT` WHEN sugar, `IN`/`NOT IN`, boolean `WHEN TRUE/FALSE`, `USING MODEL` after `INTO` |
| `internal/stdlib/stdlib.go` | `len_val()` — polymorphic length for string / JSON array / JSON object |
| `internal/executor/executor.go` | Six new SPL 3.0 exception constants; STORAGE wiring (`StorageConns` map on `WorkflowState`, open/close lifecycle, subscript read/write routing) |
| `internal/config/config.go` | `storage_dir`, `log_level`, `log_console` fields; defaults aligned with Python (`max_llm_calls`→25, `max_total_tokens`→100000) |
| `internal/executor/planner.go` | `WorkflowStep`, `WorkflowBranch`, `WorkflowPlan` types; `PlanWorkflow` / `PlanProcedure` methods |
| `cmd/docrag_cmd.go` | `readDocFile` + `extractPDFText` — PDF text via `pdftotext` subprocess (poppler) with graceful fallback |

### New files
| File | Purpose |
|---|---|
| `internal/storage/storage_conn.go` | SQLite-backed STORAGE key-value conn; DuckDB/Postgres stubs with actionable error messages |
| `internal/ir/ir.go` | Full AST→JSON serialization (`ProgramToJSON`, `MarshalProgram`, `MarshalProgramIndent`) compatible with Python `spl.ir` |

### Skipped (deferred)
Items #5–#8 (Bedrock, Gemini CLI, Vertex AI, Azure OpenAI adapters) and #10 (their config sections) are open for a future sync pass.
