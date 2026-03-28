# SPL20.go — Design Decisions

## 1. Purpose: Go Port of SPL 2.0

SPL 2.0 is a declarative, SQL-inspired agentic workflow language for LLMs.
The canonical implementation is Python (`digital-duck/SPL20`).

This repository (`digital-duck/SPL20.go`) is a Go port with one primary motivation:
**single-binary deployment alongside Momagrid**, which is also implemented in Go.
No Python runtime, no virtual environment, no pip install — just `gspl`.

---

## 2. Python First, Go Second

**Decision:** Python (`SPL20`) is the primary development environment.
Go (`SPL20.go`) receives features only after they have stabilized in Python.

**Rationale:**

- Python allows rapid iteration: grammar changes, new adapters, new stdlib functions,
  new recipe categories can be tried and adjusted in hours.
- Go is strict and verbose — experimenting in Go first slows down innovation.
- Once a feature has been validated by real cookbook runs (37+ recipes, 100% pass rate),
  it is worth the cost of a careful Go port.

**Workflow:**

```
Python SPL (spl)              Go SPL (gspl)
──────────────────            ──────────────────
experiment fast      ──►      port when stable
add new recipe       ──►      benchmark after bake-in
tweak grammar        ──►      lock down in lexer/parser
try new adapter      ──►      add Go adapter once proven
```

---

## 3. Command Name: `spl` binary, `gspl` alias

**Decision:** The Go binary is compiled as `spl` (same name as the Python CLI).
Users distinguish the two runtimes via a shell alias:

```bash
alias gspl='/path/to/SPL20.go/spl'   # Go runtime
# 'spl' continues to resolve to the Python CLI
```

**Rationale:**

- Avoids name collision on machines where both runtimes are installed.
- The alias approach is maximally flexible: the user controls which binary
  each name points to per machine, per session, or per benchmark run.
- Keeps cobra help text and error messages consistent (`spl run`, `spl validate`).
- The `gspl version` output prints `Go runtime` to make the active runtime unambiguous.

**Side-by-side benchmark pattern:**

```bash
time spl  run recipe.spl --adapter ollama   # Python runtime
time gspl run recipe.spl --adapter ollama   # Go runtime
```

Same `.spl` file, same Momagrid hub, two runtimes — accuracy diff + wall-clock diff
in one comparison.

---

## 4. No Streaming (by Design)

**Decision:** Neither the Ollama nor Momagrid adapter streams tokens.
All LLM calls are blocking: submit → wait → return full response.

**Rationale:**

Momagrid is designed for **batch inference** across a LAN grid of GPU nodes.
Streaming requires a persistent connection from hub to the calling client,
which conflicts with the hub-and-spoke task-queue model (PENDING → COMPLETE).

Streaming is a future option for dedicated single-node latency-sensitive use cases,
not a goal for the distributed grid.

---

## 5. Adapter Architecture

The `Adapter` interface is minimal — three methods:

```go
type Adapter interface {
    Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error)
    CountTokens(text, model string) int
    ListModels() []string
    Name() string
}
```

Adding a new adapter requires implementing these methods only. Current adapters:

| Adapter | Protocol | Auth |
|---|---|---|
| `echo` | Returns prompt as-is — deterministic testing | none |
| `ollama` | POST `/v1/chat/completions` to local Ollama | none |
| `momagrid` | POST `/tasks` → poll `GET /tasks/{id}` | optional API key |
| `anthropic` | POST `/v1/messages` (Anthropic Messages API) | `ANTHROPIC_API_KEY` |
| `claude_cli` | `os/exec claude -p` subprocess | Claude Code subscription |
| `openrouter` | POST `/api/v1/chat/completions` (OpenAI-compatible) | `OPENROUTER_API_KEY` |

See [ROADMAP.md](ROADMAP.md) for adapters planned or in progress.

---

## 6. Momagrid Integration

Momagrid (`digital-duck/momagrid`) is the hub-and-spoke LAN inference grid,
implemented in Go. The SPL20.go Momagrid adapter speaks the same REST protocol
as the Python SPL 2.0 adapter:

1. `POST /tasks` — submit task, receive `task_id`
2. `GET  /tasks/{id}` — poll until status is `COMPLETE` or `FAILED`
3. On transient errors (EOF, connection refused) — retry up to 3 times

**LAN benchmark results (2026-03-27):**

| Config              | Nodes | Workers | Pass Rate | Wall Clock | Speedup |
|---------------------|-------|---------|-----------|------------|---------|
| 1-GPU (Ollama)      | 1     | 1       | 37/37     | 1197.6s    | 1.0×    |
| 2-GPU (Momagrid)    | 2     | 5       | 37/37     | 660.4s     | 1.8×    |
| 3-GPU (Momagrid)    | 3     | 10      | 37/37     | 383.7s     | 3.1×    |

These results were produced with the Python SPL runtime. The Go runtime is
expected to match or exceed these numbers once validated on the 5-node grid.

---

## 7. Stability Guarantee

The Go port targets **feature parity** with the Python runtime for the 40-recipe
SPL Cookbook (`digital-duck/Cookbook-of-SPL-Recipes`). Any recipe that passes
`spl run` must also pass `gspl run` on the same adapter.

Divergences between the two runtimes are bugs in the Go port, not design choices.

For the current feature completion status and what is planned next, see [ROADMAP.md](ROADMAP.md).

---

## 8. ChromaDB + Ollama Embedding Architecture (RAG)

Both Code-RAG and Doc-RAG use the same stack:

- **Vector store:** ChromaDB (running as a local Docker container or service)
- **Embedding model:** Ollama (`nomic-embed-text` default, 768-dim)
- **Protocol:** ChromaDB REST API (no external Go SDK required)

### Prerequisites

```bash
# Start ChromaDB
docker run -p 8000:8000 chromadb/chroma

# Pull embedding model
ollama pull nomic-embed-text
```

### Collections

| Collection | Purpose |
|---|---|
| `spl_doc_rag` | Document chunks for prompt context injection |
| `spl_code_rag` | (description, SPL source) pairs for text2spl |

### CLI Usage

```bash
# Doc-RAG: index a document, then query
spl doc-rag add my_document.txt
spl doc-rag query "how to handle errors" --top-k 5

# Code-RAG: index the cookbook, then query
spl code-rag import --cookbook-dir ./cookbook
spl code-rag query "summarize a long article" --show-spl

# text2spl: compile natural language to SPL
spl text2spl "summarize a customer email and classify its sentiment" --mode workflow
spl text2spl "translate text to French" --mode prompt -o translate.spl
```

### SQLite Memory Store

The Go runtime uses `~/.spl/memory.db` (SQLite) instead of `~/.spl/memory.json`.
The database is created automatically on first use. Schema:

- `kv_store` — persistent key-value store for `STORE @var IN memory.<key>`
- `prompt_cache` — optional prompt result caching with expiry support
