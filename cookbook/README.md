# SPL 2.0 Cookbook — spl-go edition

This directory is the **Go runtime companion** to the recipes in
[digital-duck/SPL20](https://github.com/digital-duck/SPL20) (`cookbook/` folder).

All `.spl` recipe files run identically under both runtimes.
You do not need to copy or modify any recipe — just clone the SPL20 repo
and point the `spl` command at the Go binary.

---

## The key difference: no virtual environment

| | Python `spl` | Go `spl-go` |
|---|---|---|
| Install | `conda create -n spl` + `pip install spl` | `go build` → one binary |
| Activate | `conda activate spl` every session | `alias spl=~/bin/spl-go` once |
| Run | `spl run recipe.spl` | `spl run recipe.spl` (same command) |
| Uninstall | `conda remove -n spl --all` | `rm ~/bin/spl-go` |

The alias makes both runtimes CLI-compatible — recipes run identically under both.

---

## Quick start

### 1. Build the Go binary

```bash
git clone git@github.com:digital-duck/SPL20.go.git
cd SPL20.go
go build -o ~/bin/spl-go .
```

### 2. Point `spl` at the Go binary

```bash
# Add to ~/.bashrc or ~/.zshrc for persistence
alias spl=~/bin/spl-go

# Verify
spl version
# spl-go — SPL 2.0 Go runtime v0.1.0
```

### 3. Clone the SPL20 repo and navigate to its cookbook

```bash
git clone git@github.com:digital-duck/SPL20.git
cd SPL20/cookbook
```

### 4. Run a single recipe

```bash
spl run 01_hello_world/hello.spl --adapter ollama -m gemma3
```

### 5. Build the cookbook batch runner

```bash
# Build spl-run once (replaces run_all.py — pure Go, no Python needed)
go build -o ~/bin/spl-run /path/to/SPL20.go/cookbook/
```

### 6. Run the full cookbook

```bash
# Sequential (single node)
spl-run --adapter ollama

# Parallel on Momagrid LAN grid
spl-run --adapter momagrid --workers 10

# Log output to file
spl-run --adapter ollama 2>&1 | tee out/run_$(date +%Y%m%d_%H%M%S).md

# Browse the catalog
spl-run --list
spl-run --catalog
spl-run --catalog --category agentic
spl-run --ids 04,08,10-13 --adapter ollama -m gemma3

# Point at a non-default SPL20 checkout
spl-run --spl-repo /path/to/SPL20/cookbook --adapter ollama
# or set once: export SPL20_REPO=/path/to/SPL20
```

No `conda activate`. No `pip install`. No venv.

---

## Recipe compatibility

All 37 active recipes run unchanged under `spl-go`:

| Recipes | Status | Notes |
|---|---|---|
| 01–21 | Active, approved | Run as-is |
| 23–37 | Active, new | Run as-is |
| 22 | Active — **spl-go native** | See [recipe 22](#recipe-22-text2spl-demo) below |
| 38 — bedrock | **Inactive** | Requires AWS credentials — port pending cloud funding |
| 39 — vertex | **Inactive** | Requires GCP credentials — port pending cloud funding |
| 40 — azure_openai | **Inactive** | Requires Azure credentials — port pending cloud funding |

Recipes 38/39/40 are already `is_active: false` in `cookbook_catalog.json` —
`run_all.py` skips them automatically.

---

## Recipe 22 — text2spl demo

Recipe 22 in the Python cookbook launches via `bash text2spl_demo.sh`, which
calls the Python `spl text2spl` command. The Go runtime has `text2spl` built in
natively — no shell wrapper needed.

This repo ships a `spl-go`-native version of the script:

```bash
# From the SPL20/cookbook root:
bash /path/to/SPL20.go/cookbook/22_text2spl_demo/text2spl_demo.sh ollama gemma3
```

Or run `text2spl` directly:

```bash
spl text2spl "summarize a document in three bullet points" --adapter ollama -m gemma3
spl text2spl "build a review agent that refines text" --mode workflow -o review.spl
spl text2spl "classify user intent" --execute -p text="I need help with billing"
```

---

## Adapters

All adapters that work with `spl` (Python) work identically with `spl-go`:

```bash
spl run recipe.spl --adapter ollama -m gemma3
spl run recipe.spl --adapter momagrid -m gemma3
spl run recipe.spl --adapter anthropic -m claude-sonnet-4-6
spl run recipe.spl --adapter claude_cli
spl run recipe.spl --adapter openai -m gpt-4o
spl run recipe.spl --adapter deepseek -m deepseek-chat
spl run recipe.spl --adapter qwen -m qwen-plus
spl run recipe.spl --adapter openrouter -m "meta-llama/llama-3.3-70b-instruct"
spl run recipe.spl --adapter echo        # dry-run, no LLM
```

---

## Parallel execution with Momagrid

`spl-go` adds a `--workers` flag for goroutine-based parallel execution of
independent workflow steps within a single recipe:

```bash
# Recipe-level parallelism (independent steps run concurrently)
spl run recipe.spl --adapter ollama --workers 4

# Grid-level parallelism (tasks distributed across GPU nodes)
python cookbook/run_all.py --adapter momagrid --workers 10
```

### Momagrid benchmark (Python runtime, 2026-03-27)

| Config | Nodes | Workers | Pass | Wall clock | Speedup |
|---|---|---|---|---|---|
| 1-GPU Ollama | 1× GTX 1080 Ti | 1 | 37/37 | 1197.6s | 1.0× |
| 2-GPU Momagrid | 2× GTX 1080 Ti | 5 | 37/37 | 660.4s | 1.8× |
| 3-GPU Momagrid | 3× GTX 1080 Ti | 10 | 37/37 | 383.7s | **3.1×** |

Go runtime benchmark on 5-node grid pending (2026-03-30).

---

## Configuration

`spl-go` reads `~/.spl/config.yaml`. Initialize it once:

```bash
spl init
```

Then optionally set defaults:

```bash
spl config set adapter ollama
spl config set model gemma3
```

For Momagrid:

```bash
export MOMAGRID_HUB_URL=http://192.168.0.184:9000
spl run recipe.spl --adapter momagrid
```

---

## Why no venv?

The Go runtime compiles to a single static binary with no runtime dependencies.
This is intentional — see [docs/DESIGN.md](../docs/DESIGN.md) for the full
design rationale.

The Python `spl` runtime remains the primary development environment where new
features are prototyped first. Once stable, they are ported to `spl-go`.
Use Python `spl` when you need RAG, the Streamlit UI, or cloud adapters
(bedrock, vertex, azure_openai) not yet ported to Go.

See [docs/ROADMAP.md](../docs/ROADMAP.md) for the current feature parity status.
