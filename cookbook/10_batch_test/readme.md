# Recipe 10 — Batch Test

Automated testing of cookbook recipes across multiple Ollama models. Perfect for CI/CD, model benchmarking, and regression testing.

Two implementations are provided — choose the one that fits your workflow:

| | `batch_test.spl` | `batch_test.sh` |
|---|---|---|
| Style | Pure SPL, CTE fan-out | Bash nested loops |
| Execution | Parallel (all combinations at once) | Sequential |
| Report | LLM-generated PASS/FAIL summary | Shell arithmetic |
| Best for | SPL pipelines, in-workflow testing | CI/CD, shell scripts |

---

## SPL version (`batch_test.spl`)

Uses parallel CTEs to fan out all recipe × model combinations in a single workflow, then generates a structured report.

```bash
# Default models (gemma3, llama3.2)
spl-go run cookbook/10_batch_test/batch_test.spl --adapter ollama

# Custom models
spl-go run cookbook/10_batch_test/batch_test.spl --adapter ollama \
    model_1=gemma3 model_2=mistral
```

### Parameters

| Parameter | Default    | Description   |
|-----------|------------|---------------|
| `model_1` | `gemma3`   | First model   |
| `model_2` | `llama3.2` | Second model  |

### How it works

Six CTEs (3 recipes × 2 models) run in parallel. All results are collected into variables, then a `summarize_results` function generates the PASS/FAIL report in one pass.

---

## Bash version (`batch_test.sh`)

Sequential nested loop — good for CI/CD where shell exit codes matter.

```bash
# Full test: all recipes x default models (gemma3, llama3.2)
bash cookbook/10_batch_test/batch_test.sh

# Single model
MODELS="gemma3" bash cookbook/10_batch_test/batch_test.sh

# Dry run with echo adapter (no Ollama needed)
ADAPTER=echo bash cookbook/10_batch_test/batch_test.sh

# Custom model set
MODELS="gemma3 llama3.2 phi3" bash cookbook/10_batch_test/batch_test.sh \
    2>&1 | tee cookbook/out/10_batch_test-$(date +%Y%m%d_%H%M%S).md 
```

Output:
```
==============================================
  SPL 3.0 Batch Test
==============================================
  Adapter: ollama
  Models:  gemma3 llama3.2
  Time:    Wed Mar 18 2026
==============================================

>>> Model: gemma3
----------------------------------------------
  PASS  01_hello_world/hello.spl
  PASS  02_ollama_proxy/proxy.spl
  PASS  03_multilingual/multilingual.spl

>>> Model: llama3.2
----------------------------------------------
  PASS  01_hello_world/hello.spl
  PASS  02_ollama_proxy/proxy.spl
  PASS  03_multilingual/multilingual.spl

==============================================
  Results: 6/6 passed, 0 failed
==============================================
```
