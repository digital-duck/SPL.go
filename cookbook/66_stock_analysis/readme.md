# Recipe 65 — Mixed-Regime Stock Analysis

**Pattern:** `CREATE TOOL_API` (deterministic Python) + `CREATE FUNCTION` (LLM) + `WORKFLOW`

## What this demonstrates

This recipe is the canonical example of the **2×2 hybrid matrix** introduced in SPL 3.0:

| | Design time | Run time |
|---|---|---|
| **Deterministic** | `CREATE TOOL_API` — LLM *writes* Python | `CALL` → Python `exec()` |
| **Probabilistic** | `CREATE FUNCTION` — LLM *writes* prompt template | `GENERATE` → LLM inference |

**Regime principle (physics analogy):** Even though quantum mechanics is more fundamental than
classical mechanics, we still use classical mechanics to predict a missile trajectory. Similarly,
using `GENERATE` (LLM) for deterministic data fetch or arithmetic is a category error — use
the right tool for the right job.

## Architecture

```
CALL get_item()       → deterministic: Python string split   (no LLM)
CALL fetch_ohlcv()    → deterministic: Python HTTP fetch     (no LLM)
CALL compute_metrics() → deterministic: Python arithmetic    (no LLM)
GENERATE interpret_metrics() → probabilistic: LLM analysis   (uses LLM)
GENERATE synthesize_report() → probabilistic: LLM synthesis  (uses LLM)
```

## Run

```bash
# Default: AAPL, MSFT, GOOGL — last 30 days
spl-go run cookbook/66_stock_analysis/stock_analysis.spl \
    --adapter ollama --model gemma3

# Custom tickers and window
spl-go run cookbook/66_stock_analysis/stock_analysis.spl \
    --adapter ollama --model gemma3 \
    --param tickers="TSLA, NVDA, AMD" \
    --param days=60 

# Using cloud adapter
spl-go run cookbook/66_stock_analysis/stock_analysis.spl \
    --adapter claude_cli --param tickers="GDX,SILJ, WEAT,UNG"

# Using cloud adapter
spl-go run cookbook/66_stock_analysis/stock_analysis.spl \
    --adapter openrouter --model qwen/qwen3-235b-a22b \
    --param tickers="SPY,QQQ,IWM"
```

## Compile to other targets

```bash
# LangGraph (Python) — TOOL_API bodies emitted verbatim as module-level helpers
spl-go splc cookbook/66_stock_analysis/stock_analysis.spl --target langgraph

# Go — TOOL_API stubs with Python reference comment (port manually)
spl-go splc cookbook/66_stock_analysis/stock_analysis.spl --target go

# TypeScript — TOOL_API stubs with Python reference comment
spl-go splc cookbook/66_stock_analysis/stock_analysis.spl --target ts
```

## Key learning points

1. **`CREATE TOOL_API` is self-contained** — the Python implementation lives in the `.spl`
   file itself, alongside the workflow. No separate `tools.py` needed.

2. **Zero LLM calls for data/math** — `fetch_ohlcv`, `compute_metrics`, `get_item` run purely
   in Python via `exec()`. Only the final `interpret_metrics` and `synthesize_report` calls
   hit the LLM.

3. **`spl2mmd` renders distinct shapes** — TOOL_API-backed CALL nodes appear in **green**;
   LLM GENERATE nodes in **blue**; sub-workflow CALL nodes in **amber**.

4. **Promotion to library** — promote `fetch_ohlcv` and `compute_metrics` to the shared
   library so any workflow can reuse them without copy-pasting:
   ```bash
   spl-go tool-api promote cookbook/66_stock_analysis/stock_analysis.spl --name finance_tools
   spl-go tool-api list
   ```
