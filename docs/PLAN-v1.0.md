# SPL.go Consolidation Plan — v1.0

**Goal:** Consolidate SPL v1.0, v2.0, and v3.0 (Python) into a single `spl-go` binary at `digital-duck/SPL.go`.

---

## Current State of SPL.go

SPL.go is a production-quality Go port of SPL v2.0. It already implements the full core language:

- PROMPT, WORKFLOW, PROCEDURE
- GENERATE...INTO @var, @variable assignment
- WHILE loops, EVALUATE...WHEN branching
- CALL proc(), EXCEPTION...WHEN handlers
- text2spl, RAG (doc + code), memory, config

**Gaps vs SPL v2.0 Python:**

| Missing | Category |
|---|---|
| `google` adapter (Gemini / Vertex AI) | Adapter |
| `bedrock` adapter (AWS Bedrock) | Adapter |
| `azure_openai` adapter | Adapter |
| `spl ui` (Streamlit Knowledge Studio) | UI — **skip, not applicable to Go** |

**Gaps vs SPL v3.0 Python:**

| Missing | Category |
|---|---|
| `IMPORT 'file.spl'` | Language |
| `CALL PARALLEL ... END` | Language |
| `IMAGE`, `AUDIO`, `VIDEO` types | Type system |
| Codec layer (base64, WAV, video frames) | Runtime |
| Multimodal adapter layer | Adapter |
| `liquid` adapter | Adapter |
| `snap` adapter (Ubuntu 26.04, future) | Adapter |
| Workflow registry (in-process + hub-backed) | Runtime |
| Hub-to-Hub peering | Infrastructure |

---

## Phased Plan

### Phase 1 — Complete v2.0 Parity (3 missing adapters) — **DEFERRED**

> **[wen] delay until needed**

Add the three cloud adapters that are in the Python v2.0 runtime but absent from SPL.go.
All three follow the existing `Adapter` interface (`Generate`, `CountTokens`, `ListModels`, `Name`).

**Work items (when needed):**

1. `internal/adapter/google.go` — Gemini REST API or Vertex AI endpoint
2. `internal/adapter/bedrock.go` — AWS Bedrock Converse API
3. `internal/adapter/azure_openai.go` — Azure OpenAI
4. Register all three in `adapter.go` factory `New()`
5. Add per-adapter config sections to `~/.spl/config.yaml` defaults

See ROADMAP.md for tracking.

---

### Phase 2 — Port v3.0 Core Language Features — ✅ DONE (2026-04-11)

Ported the v3.0 language additions that are **not** multimodal-dependent.

#### 2a. `IMPORT 'file.spl'` ✅

- New lexer token: `IMPORT`
- New AST node: `ImportStatement { Path string }`
- Parser: handles `IMPORT 'path'` at the top level of a program
- Executor: resolves path relative to calling file's directory (`SourceDir`), parses imported file, merges `CREATE FUNCTION`, `PROCEDURE`, and `WORKFLOW` definitions into the executor registry
- Transitive imports supported (imported files can themselves `IMPORT`)

#### 2b. `CALL PARALLEL ... END` ✅

- New AST node: `CallParallelStatement { Branches []CallBranch }`
- Parser: handles multi-branch syntax with optional `INTO @var` per branch
- Executor: snapshots parent variable scope → fans out goroutines via existing `runParallel` → merges `INTO @var` results and metrics back to parent
- Each branch is fully isolated: reads parent snapshot, writes only its own `TargetVariable`
- `CALL` now also dispatches registered `WORKFLOW` definitions (not just `PROCEDURE`)

#### 2c. Extended type system ✅

- `IMAGE`, `AUDIO`, `VIDEO` added as lexer tokens and as valid `INPUT`/`OUTPUT` parameter types in the parser
- Values held as strings at runtime (file path or `data:{mime};base64,{data}`)
- Fully type-safe: v3.0 `.spl` files with media-typed params parse and validate cleanly

**Q4 — CALL PARALLEL scope:** snapshot-per-branch (no shared state) ✅ implemented as answered
**Q5 — IMPORT path resolution:** relative to calling file's directory ✅ implemented as answered

---

### Phase 3 — Multimodal Support (v3.0) — PENDING

Port the multimodal codec layer and multimodal adapter interface. Deferred until SPL30 stabilizes.

**Work items (when SPL30 is stable):**

1. `internal/codec/` — image (Go stdlib), audio (`go-wav`), video (ffmpeg subprocess)
2. `MultimodalAdapter` interface embedding `Adapter`
3. `ContentBlock` type in `internal/adapter/adapter.go`
4. Upgrade `anthropic`, `openai`, `google` (Phase 1) to implement `MultimodalAdapter`
5. `internal/adapter/liquid.go` — Liquid inference adapter
6. `internal/adapter/snap.go` — Ubuntu AI snap stub

---

### Phase 4 — Workflow Registry + Hub Peering (v3.0 Advanced) — PENDING

Deferred until Phases 1–3 are complete and Momagrid hub API is stable.

**Work items:**

1. In-process workflow registry (`internal/registry/`)
2. Hub-backed registry (REST calls to Momagrid hub)
3. Hub-to-Hub peering (design work needed)

---

## Open Questions Summary

| # | Question | Answer |
|---|---|---|
| Q1 | Google adapter: Gemini REST vs Vertex AI vs both as separate adapters? | [wen] delay until needed |
| Q2 | AWS Bedrock: which model families at launch? (Claude, Nova, Llama, Titan, all?) | [wen] delay until needed |
| Q3 | Azure OpenAI: API version to target? (`2024-02-01` GA or latest preview?) | [wen] delay until needed |
| Q4 | `CALL PARALLEL` scope: snapshot-per-branch (no shared state) or shared read-only? | [wen] build — implemented as snapshot-per-branch |
| Q5 | `IMPORT` path resolution: relative to calling file, or configurable search path? | [wen] build — implemented as relative to calling file |

---

## File Change Map (Phase 2 — completed 2026-04-11)

```
internal/lexer/lexer.go        [DONE] — IMPORT, PARALLEL, IMAGE, AUDIO, VIDEO tokens
internal/ast/nodes.go          [DONE] — ImportStatement, CallParallelStatement, CallBranch
internal/parser/parser.go      [DONE] — IMPORT, CALL PARALLEL parsing; media type keywords
internal/executor/executor.go  [DONE] — registerProgram(), execImport(), execCallParallel(),
                                         Workflows registry, CALL → workflow dispatch,
                                         SourceDir field
cmd/run.go                     [DONE] — exec.SourceDir = filepath.Dir(filename)

Tests added:
  internal/lexer/lexer_test.go   — 5 new keyword cases
  internal/parser/parser_test.go — TestImportStatement, TestImportBeforeWorkflow,
                                    TestCallParallelStatement, TestCallParallelNoInto,
                                    TestMultimodalParamTypes
  internal/executor/executor_test.go — TestCallParallelTwoProcedures,
                                        TestCallParallelResultsIsolated,
                                        TestImportLoadsDefinitions
```
