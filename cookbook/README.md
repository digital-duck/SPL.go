# SPL Cookbook — spl-go edition

This directory is the **Go runtime companion** to recipes in both:
- [digital-duck/SPL20](https://github.com/digital-duck/SPL20) (`cookbook/`) — SPL 2.0 recipes
- [digital-duck/SPL30](https://github.com/digital-duck/SPL30) (`cookbook/`) — SPL 3.0 recipes

All `.spl` recipe files run identically under both the Python and Go runtimes.
You do not need to copy or modify any recipe — just point `spl-go` at the recipe file.

---

## The key difference: no virtual environment

| | Python `spl-go` | Go `spl-go` |
|---|---|---|
| Install | `conda create -n spl3` + `pip install spl-llm` | `go build` → one binary |
| Activate | `conda activate spl3` every session | none |
| Run | `spl-go run recipe.spl` | `spl-go run recipe.spl` |
| Uninstall | `conda remove -n spl-go --all` | `rm ~/.local/bin/spl-go` |

---

## Quick start

### 1. Build and install the Go binary

```bash
git clone git@github.com:digital-duck/SPL.go.git
cd SPL.go
go build -o ~/.local/bin/spl-go .
```

### 2. Verify

```bash
spl-go version
# spl-go — SPL Go runtime v0.x.x
```

### 3. Run a SPL 2.0 recipe

```bash
cd ~/projects/digital-duck/SPL20
spl-go run cookbook/01_hello_world/hello.spl --adapter ollama -m gemma3
```

### 4. Run a SPL 3.0 recipe

```bash
cd ~/projects/digital-duck/SPL30
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter ollama \
    --param writer_model="gemma3" \
    --param critic_model="gemma3"
```

---

## Recipe compatibility

### SPL 2.0 cookbook (SPL20/cookbook)

| Recipes | Status | Notes |
|---|---|---|
| 01–21 | Active, approved | Run as-is |
| 23–37 | Active, new | Run as-is |
| 22 | Active — **spl-go native** | See [recipe 22](#recipe-22-text2spl-demo) below |
| 38 — bedrock | **Inactive** | Requires AWS credentials |
| 39 — vertex | **Inactive** | Requires GCP credentials |
| 40 — azure_openai | **Inactive** | Requires Azure credentials |

### SPL 3.0 cookbook (SPL30/cookbook)

| Recipe | Description | spl-go status | Notes |
|---|---|---|---|
| 05 `self_refine` | CALL sub-workflow, WHILE, EVALUATE | `[DONE]` ¹ | ollama verified 2026-04-14 |
| 50 `code_pipeline` | CALL chain, WHILE @item IN @items | `[DONE]` ¹ | echo verified |
| 51–62 | Multimodal (IMAGE/AUDIO/VIDEO) | `[TODO]` | codec pipeline not yet ported |
| 63 `parallel_code_review` | CALL PARALLEL (3 concurrent reviews) | `[DONE]` ¹ | echo verified |
| 64 `parallel_news_digest` | CALL PARALLEL (3 topics, merge) | `[DONE]` ¹ | echo verified |

¹ Parser fixes merged 2026-04-14. Run `spl-go run ... --adapter echo` for NDD oracle (deterministic, no LLM needed).

---

## Recipe 22 — text2spl demo

Recipe 22 in the Python cookbook launches via `bash text2spl_demo.sh`. The Go
runtime has `text2spl` built in natively — no shell wrapper needed.

```bash
spl-go text2spl "summarize a document in three bullet points" --adapter ollama -m gemma3
spl-go text2spl "build a review agent that refines text" --mode workflow -o review.spl
spl-go text2spl "classify user intent" --execute --param text="I need help with billing"
```

---

## Adapters

```bash
spl-go run recipe.spl --adapter ollama -m gemma3
spl-go run recipe.spl --adapter momagrid -m gemma3
spl-go run recipe.spl --adapter anthropic -m claude-sonnet-4-6
spl-go run recipe.spl --adapter claude_cli
spl-go run recipe.spl --adapter openai -m gpt-4o
spl-go run recipe.spl --adapter deepseek -m deepseek-chat
spl-go run recipe.spl --adapter qwen -m qwen-plus
spl-go run recipe.spl --adapter openrouter -m "meta-llama/llama-3.3-70b-instruct"
spl-go run recipe.spl --adapter echo        # dry-run / NDD oracle, no LLM
```

---

## Parallel execution

`spl-go` supports goroutine-based parallel execution via `--workers` and the
`CALL PARALLEL` construct:

```bash
# CALL PARALLEL branches run concurrently (recipe 63, 64)
spl-go run cookbook/63_parallel_code_review/parallel_code_review.spl \
    --adapter ollama --param model="gemma4:e4b" --param code="..."

# Recipe-level parallelism for independent workflow steps
spl-go run recipe.spl --adapter ollama --workers 4
```

### Momagrid benchmark (Python runtime, 2026-03-27)

| Config | Nodes | Workers | Pass | Wall clock | Speedup |
|---|---|---|---|---|---|
| 1-GPU Ollama | 1× GTX 1080 Ti | 1 | 37/37 | 1197.6s | 1.0× |
| 2-GPU Momagrid | 2× GTX 1080 Ti | 5 | 37/37 | 660.4s | 1.8× |
| 3-GPU Momagrid | 3× GTX 1080 Ti | 10 | 37/37 | 383.7s | **3.1×** |

Go runtime benchmark on multi-node grid pending.

---

## Configuration

```bash
spl-go init                        # create ~/.spl/config.yaml
spl-go config set adapter ollama
spl-go config set model gemma3
```

For Momagrid:

```bash
export MOMAGRID_HUB_URL=http://192.168.0.184:9000
spl-go run recipe.spl --adapter momagrid
```

---

## Why no venv?

The Go runtime compiles to a single static binary with no runtime dependencies.
The Python `spl-go` runtime remains the primary development environment where new
features are prototyped first. Once stable, they are ported to `spl-go`.

See [docs/ROADMAP.md](../docs/ROADMAP.md) for current feature parity across
Python, Go, and TypeScript runtimes.
