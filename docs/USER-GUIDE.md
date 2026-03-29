# spl-go User Guide — SPL 2.0 Go Runtime

## Table of Contents

1. [Installation](#1-installation)
2. [Your First SPL Program](#2-your-first-spl-program)
3. [SPL Language Basics](#3-spl-language-basics)
4. [Running Programs](#4-running-programs)
5. [Adapters](#5-adapters)
6. [Configuration](#6-configuration)
7. [Memory Store](#7-memory-store)
8. [Document RAG](#8-document-rag-doc-rag)
9. [Code RAG](#9-code-rag)
10. [text2spl — Generate SPL from Natural Language](#10-text2spl)
11. [Momagrid — Distributed Inference](#11-momagrid)
12. [Side-by-Side Benchmarking with Python spl](#12-benchmarking)
13. [Known Limitations](#13-known-limitations)

---

## 1. Installation

### Build from source

```bash
git clone git@github.com:digital-duck/SPL20.go.git
cd SPL20.go
go build -o ~/bin/spl-go .
```

### Set up the alias

Because the Python `spl` CLI may be installed on the same machine, the Go binary is aliased as `spl-go`:

```bash
alias spl-go=~/bin/spl-go          # add to ~/.bashrc or ~/.zshrc
```

Both `spl` (Python) and `spl-go` (Go) accept identical `.spl` files and flags — this makes side-by-side comparison trivial.

### Verify

```bash
spl-go version
# spl-go — SPL 2.0 Go runtime v0.1.0

spl-go adapters
# echo, ollama, momagrid, anthropic, claude_cli
```

### Optional services

| Service | Purpose | Start command |
|---|---|---|
| Ollama | Local LLM inference | `ollama serve` |
| ChromaDB | Vector store for RAG | `docker run -p 8000:8000 chromadb/chroma` |
| Momagrid | Distributed LAN grid | `mg serve` (on hub node) |

---

## 2. Your First SPL Program

Create `hello.spl`:

```sql
PROMPT hello
  GENERATE "You are a friendly assistant. Say hello and introduce yourself briefly."
  INTO @response
END
```

Run it:

```bash
spl-go run hello.spl
```

Output:
```
============================================================
Model: llama3.2
Tokens: 18 in / 54 out
Latency: 1823ms
Cost: $0.000000
------------------------------------------------------------
Hello! I'm your AI assistant, here to help you with any
questions or tasks you have. What can I do for you today?
============================================================
```

---

## 3. SPL Language Basics

SPL 2.0 is a declarative language inspired by SQL. Every program is one or more **statements**.

### PROMPT — single LLM call

```sql
PROMPT summarize
  SYSTEM "You are a concise technical writer."
  GENERATE "Summarize the following text in one paragraph: {text}"
  INTO @summary
  USING MODEL gemma3
END
```

Run with a parameter:
```bash
spl-go run summarize.spl text="Long article content here..."
```

### WORKFLOW — multi-step agentic flow

```sql
WORKFLOW draft_review
  INPUT @topic

  @draft := GENERATE "Write a 200-word blog post about {topic}."

  EVALUATE @draft
    WHEN contains: "AI" THEN
      @draft := GENERATE "Revise the draft to be less jargon-heavy: {@draft}"
    OTHERWISE
      COMMIT @draft
  END

  COMMIT @draft WITH grade="A", revised="true"
END
```

### Variable assignment

```sql
@result := GENERATE "Translate '{text}' to French."
@count  := "42"
@flag   := iif(contains(@result, "error"), "true", "false")
```

### EVALUATE — conditional branching

```sql
EVALUATE @response
  WHEN startswith: "Error" THEN
    RAISE HallucinationDetected "Model returned an error prefix"
  WHEN contains: "sorry" THEN
    @response := GENERATE "Try again without apologizing: {@response}"
  OTHERWISE
    COMMIT @response
END
```

Condition types:
- `startswith: "text"` — deterministic prefix match
- `contains: "text"` — deterministic substring match
- `LLM: "is this a valid JSON object?"` — LLM-judged condition (costs one LLM call)

### WHILE — iteration

```sql
@attempts := "0"
WHILE to_int(@attempts) < 3 AND NOT contains(@result, "done")
  @result   := GENERATE "Continue the task. Previous: {@result}"
  @attempts := to_text(to_int(@attempts) + 1)
END
```

### DO / EXCEPTION — error handling

```sql
DO
  @answer := GENERATE "Answer: {question}"
  COMMIT @answer
EXCEPTION HallucinationDetected
  @answer := "I could not reliably answer this question."
  COMMIT @answer
EXCEPTION MaxIterationsReached
  RAISE RefusalToAnswer "Exceeded retry limit"
END
```

Exception types: `HallucinationDetected`, `RefusalToAnswer`, `ContextLengthExceeded`, `ModelOverloaded`, `QualityBelowThreshold`, `MaxIterationsReached`, `BudgetExceeded`, `NodeUnavailable`

### GENERATE...INTO (fan-out with SELECT)

```sql
WITH topics AS (SELECT "quantum", "fusion", "materials")
SELECT topic
  @summary_{topic} := GENERATE "One sentence on {topic} in physics."
INTO @results
```

### Built-in functions (stdlib)

```sql
upper(@text)           lower(@text)           trim(@text)
length(@text)          substr(@text, 1, 10)   replace(@text, "old", "new")
concat(@a, " ", @b)    startswith(@text, "A") contains(@text, "error")

to_int(@s)             to_float(@s)           to_bool(@s)
abs_val(@n)            round_val(@n)          clamp(@n, "0", "100")

json_get(@json, "key") json_set(@json, "key", "val")
now_iso()              date_diff_days(@d1, @d2)
md5_hash(@text)        sha256_hash(@text)
word_count(@text)      line_count(@text)

iif(@cond, "yes", "no")   coalesce(@a, @b, "default")   nvl(@val, "fallback")
trim_turns(@history, "10") -- trim conversation to last 10 turns
```

Full stdlib reference: 47 functions across type conversion, string, pattern matching, numeric, conditional, null/empty, text aggregate, JSON, date/time, hashing, and list categories.

### STORE — persist to memory

```sql
STORE @summary IN memory.last_summary
-- Later in the same or different workflow:
@prev := memory.last_summary
```

Values persist in `~/.spl/memory.db` (SQLite).

---

## 4. Running Programs

### Basic run

```bash
spl-go run my_workflow.spl
```

### Pass parameters

```bash
# Positional KEY=VALUE
spl-go run summarize.spl topic="climate change" length="short"

# Flag form
spl-go run summarize.spl -p topic="climate change" -p length="short"
```

### Override adapter and model

```bash
spl-go run my.spl --adapter ollama -m llama3.2
spl-go run my.spl --adapter momagrid -m gemma3
spl-go run my.spl --adapter anthropic -m claude-sonnet-4-6
spl-go run my.spl --adapter claude_cli
spl-go run my.spl --adapter echo          # dry-run, no LLM calls
```

### Validate syntax without running

```bash
spl-go validate my_workflow.spl
# ✓ Valid: my_workflow.spl
```

### Explain structure

```bash
spl-go explain my_workflow.spl
# Statements: 4
# WORKFLOWs: draft_review
# PROMPTs:   (none)
# Parameters: @topic
```

---

## 5. Adapters

### ollama (default)

Requires [Ollama](https://ollama.com) running locally.

```bash
ollama serve
ollama pull llama3.2    # or gemma3, phi3, deepseek-r1, qwen3, etc.

spl-go run my.spl --adapter ollama -m gemma3
```

Config in `~/.spl/config.yaml`:
```yaml
adapters:
  ollama:
    base_url: http://localhost:11434
    default_model: llama3.2
    timeout_secs: 120
```

Override via env:
```bash
OLLAMA_BASE_URL=http://192.168.0.10:11434 spl-go run my.spl
```

### momagrid

Requires a running [Momagrid](https://github.com/digital-duck/momagrid) hub.

```bash
spl-go run my.spl --adapter momagrid -m gemma3
```

Config:
```yaml
adapters:
  momagrid:
    base_url: http://192.168.0.184:9000
    timeout_secs: 600
```

Override via env:
```bash
MOMAGRID_HUB_URL=http://192.168.0.184:9000 spl-go run my.spl --adapter momagrid
```

### anthropic

Requires `ANTHROPIC_API_KEY` environment variable.

```bash
export ANTHROPIC_API_KEY=sk-ant-...
spl-go run my.spl --adapter anthropic -m claude-sonnet-4-6
```

Available models: `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5-20251001`

### claude_cli

Uses the [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude` binary). Billed via Claude Code subscription — **$0.00 per call**.

```bash
spl-go run my.spl --adapter claude_cli
```

Config:
```yaml
adapters:
  claude_cli:
    cli_path: claude              # path to claude binary
    default_model: claude-sonnet-4-6
    timeout_secs: 300
```

### echo

Returns the assembled prompt as output. No LLM call. Useful for testing prompt construction.

```bash
spl-go run my.spl --adapter echo
```

---

## 6. Configuration

Config file: `~/.spl/config.yaml`

### View current config

```bash
spl-go config show
```

### Get / set values

```bash
spl-go config get adapter
spl-go config set adapter momagrid

spl-go config get model
spl-go config set model gemma3

spl-go config set max_llm_calls 50
spl-go config set max_total_tokens 200000
```

### Config file path

```bash
spl-go config path
# /home/user/.spl/config.yaml
```

### Full default config

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
    base_url: https://api.anthropic.com
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
  collection: spl_code_rag
  top_k: 4
  auto_capture: true

doc_rag:
  chroma_url: http://localhost:8000
  ollama_url: http://localhost:11434
  embed_model: nomic-embed-text
  collection: spl_doc_rag
```

---

## 7. Memory Store

The memory store (`~/.spl/memory.db`) is a SQLite-backed key-value store shared across all workflow runs.

### In SPL programs

```sql
-- Write
STORE @summary IN memory.last_summary
STORE @user_name IN memory.user

-- Read
@prev_summary := memory.last_summary
@name         := memory.user
```

### CLI commands

```bash
spl-go memory list                   # show all keys
spl-go memory get last_summary       # get a value
spl-go memory set greeting "Hello!"  # set a value
spl-go memory delete last_summary    # delete a key
```

---

## 8. Document RAG (doc-rag)

Doc-RAG stores and semantically searches document chunks. Use it to inject relevant context into prompts.

### Prerequisites

```bash
docker run -p 8000:8000 chromadb/chroma
ollama pull nomic-embed-text
```

### Index documents

```bash
# Add raw text
spl-go doc-rag add "ChromaDB is a vector database for AI applications."

# Add a file (text file)
spl-go doc-rag add ./report.txt

# Long documents are split on blank lines (paragraph chunking)
spl-go doc-rag add ./long_paper.txt
```

### Query

```bash
spl-go doc-rag query "what is a vector database"
spl-go doc-rag query "AI inference" --top-k 10
```

### Count

```bash
spl-go doc-rag count
# 47 documents indexed
```

### Use in SPL programs

```sql
WORKFLOW answer_question
  INPUT @question

  @context := RAG QUERY {@question} TOP 5
  @answer  := GENERATE "Given this context:\n{@context}\n\nAnswer: {question}"
  COMMIT @answer
END
```

---

## 9. Code RAG

Code-RAG stores (description, SPL source) pairs and retrieves the most relevant examples for `text2spl`.

### Prerequisites

Same as doc-rag: ChromaDB + `nomic-embed-text`.

### Index the cookbook

```bash
spl-go code-rag import --cookbook-dir /path/to/Cookbook-of-SPL-Recipes/cookbook
spl-go code-rag count
# 40 pairs indexed
```

### Add a custom pair

```bash
spl-go code-rag add "translate text to Spanish" my_translate.spl
```

### Query for examples

```bash
spl-go code-rag query "classify user intent from chat message"
spl-go code-rag query "summarize a document" --top-k 4 --show-spl
```

When Code-RAG is populated, `spl-go text2spl` automatically injects the top-k retrieved examples into the compiler system prompt, dramatically improving generation quality.

---

## 10. text2spl

`text2spl` compiles a natural language description into valid SPL 2.0 source code using an LLM.

### Basic usage

```bash
spl-go text2spl "summarize a document in three bullet points"
```

Output:
```sql
PROMPT summarize_document
  SYSTEM "You are a precise summarizer."
  GENERATE "Summarize the following document in exactly three bullet points:\n\n{document}"
  INTO @summary
END
```

### Generation modes

```bash
spl-go text2spl "classify user intent" --mode prompt    # single PROMPT
spl-go text2spl "classify user intent" --mode workflow  # multi-step WORKFLOW
spl-go text2spl "classify user intent" --mode auto      # LLM decides (default)
```

### Save to file

```bash
spl-go text2spl "translate email to French" -o translate.spl
```

### Generate and execute immediately

```bash
spl-go text2spl "say hello in three languages" --execute

# With parameters
spl-go text2spl "translate text to Spanish" --execute -p text="Good morning"
```

### Override the compiler model

```bash
# Use Ollama for compilation (free, no API key)
spl-go text2spl "draft a meeting summary" --adapter ollama -m qwen2.5-coder

# Use Anthropic directly
spl-go text2spl "draft a meeting summary" --adapter anthropic -m claude-sonnet-4-6
```

### Validation and retry

By default, `text2spl` validates the generated SPL by lexing and parsing it. If the LLM produces invalid syntax, the error is fed back for up to 2 correction attempts.

```bash
spl-go text2spl "my description" --no-validate    # skip validation
```

### Config

```yaml
text2spl:
  adapter: claude_cli           # compiler LLM adapter
  model: claude-sonnet-4-6      # compiler model
  mode: auto
  validate: true
  max_retries: 2
```

---

## 11. Momagrid — Distributed Inference

Momagrid is a hub-and-spoke LAN inference grid. Multiple GPU nodes share a task queue. SPL recipes are dispatched to whichever node is free.

### Architecture

```
spl-go run recipe.spl --adapter momagrid
       │
       ▼
  Momagrid Hub  (port 9000)
  ┌─────────────────────────┐
  │  Task Queue (SQLite)    │
  └────┬────────┬───────────┘
       │        │
  Agent duck   Agent dog   Agent cat
  GTX 1080 Ti  GTX 1080 Ti  GTX 1080 Ti
  (Ollama)     (Ollama)     (Ollama)
```

### Setup

On the hub node:
```bash
mg serve --port 9000
```

On each agent node:
```bash
mg join http://<hub-ip>:9000 --name cat --host 0.0.0.0 --port 9010
```

### Run a recipe on the grid

```bash
export MOMAGRID_HUB_URL=http://192.168.0.184:9000
spl-go run recipe.spl --adapter momagrid -m gemma3
```

### Benchmark results (2026-03-27, Python runtime)

| Config | Nodes | Workers | Pass Rate | Wall Clock | Speedup |
|---|---|---|---|---|---|
| 1-GPU (Ollama) | 1× GTX 1080 Ti | 1 | 37/37 | 1197.6s | 1.0× |
| 2-GPU (Momagrid) | 2× GTX 1080 Ti | 5 | 37/37 | 660.4s | 1.8× |
| 3-GPU (Momagrid) | 3× GTX 1080 Ti | 10 | 37/37 | 383.7s | **3.1×** |

The Go runtime targets identical results. Benchmark with `spl-go` pending 5-node grid (Monday 2026-03-30).

---

## 12. Benchmarking

Since `spl` (Python) and `spl-go` (Go) accept identical `.spl` files and flags, side-by-side comparison is trivial.

### Single recipe

```bash
time spl  run recipe.spl --adapter ollama -m gemma3
time spl-go run recipe.spl --adapter ollama -m gemma3
```

### Full cookbook

```bash
# Python runtime
time python run_all.py --adapter ollama

# Go runtime — alias spl to spl-go for the run
alias spl=~/bin/spl-go
time python run_all.py --adapter ollama
unalias spl
```

### Reporting a divergence

If a recipe passes `spl` but fails `spl-go`, file a bug with:
- The `.spl` file content
- Adapter and model used
- Output of both `spl run` and `spl-go run`

Divergences are bugs in the Go port, not design choices.

---

## 13. Known Limitations

When `spl-go` encounters an unsupported construct, it prints a clear warning to stderr and continues — it does not crash.

```
WARNING: [feature] not fully supported in spl-go (Go runtime).
Use 'spl' (Python) for this feature. See ROADMAP in docs/DESIGN.md
```

| Feature | Workaround |
|---|---|
| `STORAGE()` multi-backend (DuckDB, Postgres) | Use `STORE @var IN memory.key` instead |
| `RAG QUERY` in PROMPT body | Run `spl-go doc-rag query`, pass result as parameter |
| Adapters: openai, google, openrouter, deepseek, bedrock, azure, vertex | Use Python `spl` |
| Accurate token counting | Go uses chars/4 estimate; Python `spl` has exact counts |
| Optimizer / parallel execution plans | Go executes statements sequentially |
| Static analyzer (`spl analyze`) | Use Python `spl validate` |
| Streamlit UI (`spl ui`) | Use Python `spl ui` |
| Async / concurrent workflow steps | Use Python `spl` for asyncio workflows |

All items are on the roadmap — see [DESIGN.md](DESIGN.md).

---

## Appendix A — SPL Statement Quick Reference

| Statement | Purpose |
|---|---|
| `PROMPT name ... END` | Single LLM call |
| `WORKFLOW name ... END` | Multi-step agentic flow |
| `PROCEDURE name ... END` | Reusable sub-workflow |
| `@var := GENERATE "..."` | LLM call, store result |
| `EVALUATE @var WHEN ... END` | Conditional branching |
| `WHILE cond ... END` | Loop until condition |
| `DO ... EXCEPTION type ... END` | Error handling |
| `COMMIT @var [WITH k=v]` | Finalize workflow output |
| `RAISE ExceptionType "msg"` | Throw a named exception |
| `RETRY [WITH model=...]` | Retry last LLM call |
| `CALL tool_name(args)` | Call an external tool |
| `STORE @var IN memory.key` | Persist to SQLite memory |
| `LOGGING "message"` | Write to SPL log |
| `SELECT col FROM cte INTO @var` | Fan-out generation |

---

## Appendix B — Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `ANTHROPIC_API_KEY` | — | Anthropic API authentication |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Override Ollama server |
| `MOMAGRID_HUB_URL` | `http://localhost:9000` | Override Momagrid hub |
| `MOMAGRID_API_KEY` | — | Momagrid authentication |

---

## Appendix C — File Layout

```
~/.spl/
  config.yaml        # runtime configuration
  memory.db          # SQLite key-value + prompt cache
  logs/              # execution logs (one .md file per run)
```

ChromaDB collections are managed by the ChromaDB server (default: `http://localhost:8000`).
