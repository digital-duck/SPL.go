# Recipe 64: Parallel News Digest

**Category:** agentic / SPL 3.0  
**SPL version:** 3.0  
**Type:** TEXT → TEXT  
**LLM required:** Yes — any text model (default: `gemma3`)  
**Demonstrates:** `CALL PARALLEL`, workflow fan-out + merge, pure-text pipeline

---

## What it does

Summarises three independent news topics **concurrently** using `CALL PARALLEL`,
then synthesises all three summaries into a single morning briefing.

```
@topic_tech  ──┐
@topic_sci   ──┼── CALL PARALLEL ──► 3 summaries ──► merge ──► @digest
@topic_biz   ──┘
```

This is the canonical SPL 3.0 **fan-out → merge** pattern:

1. `CALL PARALLEL` fans out to three `summarise_single` workflow instances
2. Each branch is fully isolated — it receives a snapshot of the parent scope
3. Results flow back only via `INTO @var`
4. The parent merges the three summaries into one briefing with a final `GENERATE`

Because all three topics are independent, parallel execution saves ~2× wall-clock
time vs sequential (the merge step still costs one serial LLM call).

---

## Files

| File | Role |
|---|---|
| `parallel_news_digest.spl` | Main recipe — prompt functions + `summarise_single` sub-workflow + `parallel_news_digest` orchestrator |
| `logs-spl/` | Execution logs (auto-created on run) |

No `run.py` needed — pure SPL, runs directly with `spl-go run` or `spl-go run`.

---

## Prerequisites

```bash
ollama serve
ollama pull gemma3      # or any text model: llama3.2, phi4, etc.
```

No API keys, no external services beyond Ollama.

---

## Running

```bash
# Default topics, default model (gemma3)
spl-go run cookbook/64_parallel_news_digest/parallel_news_digest.spl

# Custom topics
spl-go run cookbook/64_parallel_news_digest/parallel_news_digest.spl \
    --param topic_tech="quantum computing and post-silicon chips" \
    --param topic_science="CRISPR gene editing clinical trials" \
    --param topic_business="electric vehicle market slowdown"

# Different model
spl-go run cookbook/64_parallel_news_digest/parallel_news_digest.spl \
    --param digest_model=llama3.2

# With spl-go
spl-go run cookbook/64_parallel_news_digest/parallel_news_digest.spl \
    --adapter ollama --param digest_model=gemma3

# Dry-run (no LLM calls — inspect prompt assembly)
spl-go run cookbook/64_parallel_news_digest/parallel_news_digest.spl \
    --adapter echo
```

---

## Expected output

```
============================================================
Workflow Status: complete
LLM Calls: 4 | Tokens: ~800 in / ~400 out
Latency: ~3000ms (parallel branches) | Cost: $0.000000
------------------------------------------------------------
Committed Output:
Good morning. Here are today's key developments across three domains.

**Technology**
Large language models continue to advance rapidly, with several labs releasing
...

**Science**
Astronomers announced ...

**Business**
Energy markets saw ...

Watch today: ...
============================================================
```

LLM calls breakdown: 3 parallel `summarise_single` calls + 1 serial `morning_briefing` merge.

---

## SPL 3.0 concepts illustrated

| Concept | Where |
|---|---|
| `CALL PARALLEL ... END` | Fan-out to 3 `summarise_single` branches |
| `CALL` dispatching a `WORKFLOW` | `summarise_single` is a WORKFLOW, not a PROCEDURE |
| Branch isolation | Each branch snapshots parent scope; only `INTO @var` writes back |
| Fan-out → merge | Parallel branches feed a single final `GENERATE` |
| Named arguments in `CALL` | `model=@model, log_dir=@log_dir` |
