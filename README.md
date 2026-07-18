# SPL.go — SPL Go Runtime (v1 · v2 · v3)

**spl-go** is the Go implementation of the Semantic Prompt Language (SPL), consolidating SPL v1.0, v2.0, and v3.0 into a single production binary. SPL is a declarative, SQL-inspired language for LLM-powered agentic workflows.

```sql
-- SPL 2.0: multi-step workflow
WORKFLOW self_refine
  INPUT: @task text, @max_iterations integer DEFAULT 3
  OUTPUT: @result text
DO
  GENERATE draft(@task) INTO @current
  @iteration := "0"
  WHILE to_int(@iteration) < @max_iterations DO
    GENERATE critique(@current) INTO @feedback
    EVALUATE @feedback
      WHEN 'satisfactory' THEN COMMIT @current
      ELSE GENERATE refine(@current, @feedback) INTO @current
    END
    @iteration := to_text(to_int(@iteration) + 1)
  END
  COMMIT @current
EXCEPTION MaxIterationsReached
  COMMIT @current WITH status='partial'
END
```

```sql
-- SPL 3.0: parallel dispatch across imported workflows
IMPORT 'lib/review.spl'
IMPORT 'lib/test.spl'

WORKFLOW pipeline
  INPUT: @code text
  OUTPUT: @report text
DO
  CALL PARALLEL
    review_code(@code) INTO @style,
    test_code(@code)   INTO @coverage
  END
  @report := concat(@style, "\n---\n", @coverage)
  COMMIT @report
END
```

```bash
spl-go run pipeline.spl code="$(cat main.go)" --adapter anthropic -m claude-sonnet-4-6
```

---

## Why spl-go?

| | Python `spl` | Go `spl-go` |
|---|---|---|
| **Purpose** | Development, experimentation | Production, single-binary deployment |
| **Deploy** | pip + venv | `go build` → one binary |
| **Momagrid** | Python adapter | Native Go (same language as Momagrid hub) |
| **Cold start** | 0.5–1.5s | <10ms |
| **Iteration** | Fast — new features land here first | Stable — proven features ported from Python |

Use `spl` to experiment. Use `spl-go` when you want a self-contained binary next to your Momagrid node.

---

## Installation

```bash
git clone git@github.com:digital-duck/SPL.go.git
cd SPL.go
go build -o ./spl-go .

ln -s ~/projects/digital-duck/SPL.go/spl-go ~/.local/bin/spl-go

```

**Requirements:** Go 1.22+

**Optional services** (for RAG and distributed inference):
```bash
ollama serve                                  # local LLM inference
docker run -p 8000:8000 chromadb/chroma       # vector store (RAG)
```

---

## Quick Start

```bash
# Validate an SPL file
spl-go validate my_workflow.spl

# Run with Ollama (default)
spl-go run my_workflow.spl -m gemma3

# Run on Momagrid LAN grid
spl-go run my_workflow.spl --adapter momagrid -m gemma3

# Run with Anthropic API
ANTHROPIC_API_KEY=sk-... spl-go run my_workflow.spl --adapter anthropic -m claude-sonnet-4-6

# Run with Claude Code CLI (zero marginal cost)
spl-go run my_workflow.spl --adapter claude_cli

# Dry-run (echo adapter — no LLM calls)
spl-go run my_workflow.spl --adapter echo

# Generate SPL from natural language
spl-go text2spl "summarize a document in three bullet points"

# Inspect what an SPL file contains
spl-go explain my_workflow.spl

# Show pre-execution resource estimates
spl-go run my_workflow.spl --plan
```

---

## SPL 3.0 Features (ported 2026-04-11)

### IMPORT — compose across files

```sql
IMPORT 'lib/shared.spl'   -- merges FUNCTION/PROCEDURE/WORKFLOW definitions

WORKFLOW main
  INPUT: @topic text
DO
  CALL shared_helper(@topic) INTO @result
  COMMIT @result
END
```

Paths resolve relative to the calling `.spl` file. Transitive imports supported.

### CALL PARALLEL — concurrent execution

```sql
CALL PARALLEL
  summarize(@doc)  INTO @summary,
  classify(@doc)   INTO @category,
  extract_kw(@doc) INTO @keywords
END
-- All three run concurrently; each branch is fully isolated
```

### Multimodal type keywords

```sql
WORKFLOW image_restyle
  INPUT:
    @photo IMAGE DEFAULT 'photo.jpg',
    @style text  DEFAULT 'oil painting'
  OUTPUT: @result IMAGE
DO
  GENERATE analyse(@photo, @style) INTO @result
  COMMIT @result
END
```

`IMAGE`, `AUDIO`, `VIDEO` are valid `INPUT`/`OUTPUT` parameter types. Full codec encoding (Phase 3) is planned once SPL30 stabilizes.

---

## Neurosymbolic Verifier Mode — `SOLVE` / `ASSERT` (ported 2026-07-13)

Mirrors the Python `spl3` verifier ladder: an LLM step proposes a solution and a
deterministic kernel checks it, so the answer is never taken on the model's word.

```bash
# Persistent python3 subprocess (default) — custom stdin/stdout REPL protocol
spl-go run verify.spl --kernel

# Real Jupyter wire protocol over ZeroMQ — supports non-Python kernelspecs (e.g. SageMath)
spl-go run verify.spl --kernel --kernel-protocol zmq --kernel-name sagemath
```

State (imports, variables, TOOL_API function defs) persists across `SOLVE`/`ASSERT`
steps within the same kernel session. `--tools <file.py>` registers `@spl_tool`-decorated
Python functions as CALL-able tools, dispatched via subprocess the same way the Python
runtime's `--tools` flag works.

`STORAGE` workflow params are backed by SQLite (`modernc.org/sqlite`) out of the box;
DuckDB/Postgres backends print install instructions rather than failing silently.

---

## Adapters

| Adapter | Description | Config |
|---|---|---|
| `ollama` | Local Ollama server (default) | `http://localhost:11434` |
| `momagrid` | Distributed LAN inference grid | `http://localhost:9000` |
| `anthropic` | Anthropic Claude API | `ANTHROPIC_API_KEY` |
| `claude_cli` | Claude Code CLI (subscription) | `claude` binary in PATH |
| `openai` | OpenAI API | `OPENAI_API_KEY` |
| `openrouter` | 100+ models via OpenRouter | `OPENROUTER_API_KEY` |
| `google` | Google Gemini API | `GOOGLE_API_KEY` |
| `deepseek` | DeepSeek API | `DEEPSEEK_API_KEY` |
| `qwen` | Alibaba Cloud DashScope | `DASHSCOPE_API_KEY` |
| `echo` | Returns prompt as output — for testing | none |

---

## Commands

```
spl-go run <file.spl> [KEY=VALUE...]   Execute an SPL program
spl-go run --plan                      Show pre-execution resource estimates
spl-go run --workers N                 Parallel step execution (N goroutines)
spl-go run --tools <file.py>           Register @spl_tool Python functions as CALL-able tools
spl-go run --allowed-tools T1 T2       Tools to allow for claude_cli adapter (e.g. WebSearch, Bash)
spl-go run --kernel                    Persistent kernel session for SOLVE/ASSERT (deterministic mode)
spl-go run --llm adapter:model         Combined adapter[:model] shorthand (spl3-compatible)
spl-go validate <file.spl>             Check syntax
spl-go explain <file.spl>              Summarize structure
spl-go text2spl "<description>"        Generate SPL from natural language
spl-go adapters                        List adapters
spl-go version                         Show runtime version
spl-go init                            Initialize ~/.spl/ and write default config.yaml

spl-go config show                     Print current config
spl-go config get <key>                Get a config value
spl-go config set <key> <value>        Set a config value
spl-go config path                     Print config file path

spl-go memory list                     List all memory keys
spl-go memory get <key>                Get a memory value
spl-go memory set <key> <value>        Set a memory value
spl-go memory delete <key>             Delete a memory entry

spl-go cache list                      List all entries in the prompt cache
spl-go cache clear                     Clear the prompt cache

spl-go doc-rag add <text_or_file>      Index a document
spl-go doc-rag query "<search>"        Semantic search
spl-go doc-rag count                   Count indexed documents

spl-go code-rag import                 Index cookbook recipes
spl-go code-rag add "<desc>" <file>    Add a (description, SPL) pair
spl-go code-rag query "<desc>"         Find similar examples
spl-go code-rag count                  Count indexed pairs
```

---

## Momagrid Benchmark (2026-03-27)

Tested on 37 SPL cookbook recipes:

| Config | Nodes | Workers | Pass Rate | Wall Clock | Speedup |
|---|---|---|---|---|---|
| 1-GPU (Ollama) | 1× GTX 1080 Ti | 1 | 37/37 | 1197.6s | 1.0× |
| 2-GPU (Momagrid) | 2× GTX 1080 Ti | 5 | 37/37 | 660.4s | 1.8× |
| 3-GPU (Momagrid) | 3× GTX 1080 Ti | 10 | 37/37 | 383.7s | **3.1×** |

Results from the Python `spl` runtime. Go runtime benchmark pending on 5-node grid.

---

## Configuration

`~/.spl/config.yaml` (auto-created on first run):

```yaml
adapter: ollama
model: ""
max_llm_calls: 100
max_total_tokens: 500000
storage_dir: .spl        # base dir for STORAGE-backed workflow params
log_level: info          # debug|info|warn|error
log_console: false

adapters:
  ollama:
    base_url: http://localhost:11434
    default_model: llama3.2
    timeout_secs: 120
  anthropic:
    default_model: claude-sonnet-4-6
    timeout_secs: 180
  claude_cli:
    cli_path: claude
    default_model: claude-sonnet-4-6
    timeout_secs: 300
  google:
    default_model: gemini-2.5-flash
    timeout_secs: 180
  momagrid:
    base_url: http://localhost:9000
    timeout_secs: 600

text2spl:
  adapter: claude_cli
  model: claude-sonnet-4-6
  mode: auto
  validate: true
  max_retries: 2

code_rag:
  enabled: true
  chroma_url: http://localhost:8000
  ollama_url: http://localhost:11434
  embed_model: nomic-embed-text
  top_k: 4

doc_rag:
  chroma_url: http://localhost:8000
  ollama_url: http://localhost:11434
  embed_model: nomic-embed-text
```

---

## Relationship to Python SPL

`spl-go` is ported from `spl3` (see `digital-duck/SPL.py`) 

---

## License

Apache 2.0 — see [LICENSE](LICENSE)
