# SPL20.go — SPL Go Runtime

- rename SPL20.go to SPL.go
- port SPL v1.0, v2.0, v3.0 into Go-lang

**spl-go** is the Go implementation of [SPL 2.0](https://github.com/digital-duck/SPL20) (Semantic Prompt Language), a declarative, SQL-inspired language for LLM-powered agentic workflows.

```sql
PROMPT greet
  GENERATE "Write a one-sentence welcome for a {topic} workshop."
  INTO @response
  USING MODEL gemma3
END
```

```bash
spl-go run greet.spl topic="machine learning"
```

---

## Why spl-go?

| | Python `spl` | Go `spl-go` |
|---|---|---|
| **Purpose** | Development, experimentation | Production, single-binary deployment |
| **Deploy** | pip + venv | `go build` → one binary |
| **Momagrid** | Python adapter | Native Go (same language as Momagrid hub) |
| **Iteration** | Fast — new features land here first | Stable — proven features ported from Python |

Use `spl` to experiment. Use `spl-go` when you want a self-contained binary next to your Momagrid node.

---

## Installation

```bash
git clone git@github.com:digital-duck/SPL20.go.git
cd SPL20.go
go build -o ~/bin/spl-go .
alias spl-go=~/bin/spl-go
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
```

---

## Adapters

| Adapter | Description | Config |
|---|---|---|
| `ollama` | Local Ollama server (default) | `http://localhost:11434` |
| `momagrid` | Distributed LAN inference grid | `http://localhost:9000` |
| `anthropic` | Anthropic Claude API | `ANTHROPIC_API_KEY` env var |
| `claude_cli` | Claude Code CLI (subscription) | `claude` binary in PATH |
| `echo` | Returns prompt as output — for testing | none |

---

## Commands

```
spl-go run <file.spl> [KEY=VALUE...]   Execute an SPL program
spl-go validate <file.spl>             Check syntax
spl-go explain <file.spl>              Summarize structure
spl-go text2spl "<description>"        Generate SPL from natural language
spl-go adapters                        List adapters
spl-go version                         Show runtime version

spl-go config show                     Print current config
spl-go config get <key>                Get a config value
spl-go config set <key> <value>        Set a config value
spl-go config path                     Print config file path

spl-go memory list                     List all memory keys
spl-go memory get <key>                Get a memory value
spl-go memory set <key> <value>        Set a memory value
spl-go memory delete <key>             Delete a memory entry

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

Results from the Python `spl` runtime. The Go runtime targets identical results.

---

## Configuration

`~/.spl/config.yaml` (auto-created on first run):

```yaml
adapter: ollama
model: ""
max_llm_calls: 100
max_total_tokens: 500000

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

## Relationship to Python SPL 2.0

`spl-go` follows a **Python-first, Go-second** discipline:

1. New features land in `digital-duck/SPL20` (Python) first
2. After validation on the cookbook benchmark, they are ported to Go
3. Any recipe that passes `spl run` must also pass `spl-go run` — divergences are bugs

See [docs/DESIGN.md](docs/DESIGN.md) for the full design rationale and [docs/USER-GUIDE.md](docs/USER-GUIDE.md) for detailed usage.

---

## License

Apache 2.0 — see [LICENSE](LICENSE)
