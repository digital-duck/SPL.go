# Recipe 26 — Prompt A/B Test

Compare two prompt variants using `EVALUATE` scoring. The workflow picks the winner automatically.
Supports pre-built experiments from a catalog or fully ad-hoc task + prompt pairs.

## What's in this recipe

| File | Purpose |
|---|---|
| `ab_test.spl` | Main SPL workflow |
| `tools.py` | Python tools: `load_experiment`, `list_experiments`, `extract_score_total`, `format_tie_result` |
| `experiments.json` | 7 pre-built A/B test scenarios |

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `experiment_id` | TEXT | `''` | Experiment ID from catalog (e.g. `neural_networks`, `standing_desk`) |
| `task` | TEXT | `''` | Shared task for ad-hoc runs (used when `experiment_id` is blank) |
| `prompt_a` | TEXT | `''` | First prompt instruction (ad-hoc runs) |
| `prompt_b` | TEXT | `''` | Second prompt instruction (ad-hoc runs) |
| `winner_threshold` | FLOAT | `1.5` | Min score difference to declare a winner (below = tie) |

Pass `experiment_id` for catalog-grounded runs (recommended).
Pass `task` + `prompt_a` + `prompt_b` for ad-hoc runs with no catalog.

## Usage

Always pass `--tools tools.py`:

```bash
# From catalog — neural networks explanation styles
spl-go run cookbook/26_ab_test/ab_test.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/26_ab_test/tools.py \
    experiment_id=neural_networks \
    2>&1 | tee cookbook/out/26_ab_test-$(date +%Y%m%d_%H%M%S).md

# From catalog — standing desk value proposition
spl-go run cookbook/26_ab_test/ab_test.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/26_ab_test/tools.py \
    experiment_id=standing_desk winner_threshold=2.0

# From catalog — code review style
spl-go run cookbook/26_ab_test/ab_test.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/26_ab_test/tools.py \
    experiment_id=code_review

# Ad-hoc — no catalog needed
spl-go run cookbook/26_ab_test/ab_test.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/26_ab_test/tools.py \
    task="Summarize the French Revolution in 3 sentences" \
    prompt_a="Be concise and factual" \
    prompt_b="Be vivid and narrative"

# Discover available experiments
spl-go run ... experiment_id=list
```

## Workflow steps

```
experiment_id  (or task + prompt_a + prompt_b)
    │
    ├─ CALL load_experiment()           ← catalog context — zero LLM cost
    │
    ├─ GENERATE run_variant_a()         ← LLM, grounded by experiment context
    ├─ GENERATE run_variant_b()         ← LLM, grounded by experiment context
    │
    ├─ GENERATE evaluate_response() × 2 ← LLM, scores each against rubric
    │
    ├─ CALL extract_score_total() × 2   ← JSON parse — zero LLM cost
    │
    └─ EVALUATE (score_a - score_b)
          WHEN > threshold  → COMMIT winner=A
          WHEN < -threshold → COMMIT winner=B
          OTHERWISE         → CALL format_tie_result() → COMMIT winner=tie
```

## Python tools (`tools.py`)

### `load_experiment(experiment_id)`
Returns a structured text block with task, prompt_a, prompt_b, expected winner, and design notes.
Grounds `run_variant_a` and `run_variant_b` with the experiment context.

### `list_experiments()`
Lists all experiment IDs, tasks, and notes from `experiments.json`.

### `extract_score_total(score_json)`
Deterministically parses the scoring rubric JSON and returns the numeric total.
Replaces `GENERATE extract_total()` — zero tokens.

### `format_tie_result(response_a, response_b, score_a_json, score_b_json)`
Formats a side-by-side tie report with per-dimension scores and both responses.
Replaces `GENERATE compare_outputs()` — zero tokens.

## Experiment catalog (`experiments.json`)

| `experiment_id` | Task | Expected winner |
|---|---|---|
| `neural_networks` | Explain how neural networks learn | audience-dependent |
| `standing_desk` | Product description for standing desk | context-dependent |
| `email_subject` | Email subject for TaskFlow launch | audience-dependent |
| `code_review` | Feedback on a Python CSV function | context-dependent |
| `error_message` | Error message for 10MB upload limit | depends on context |
| `onboarding_email` | First onboarding email for CloudDash | audience-dependent |
| `sql_explanation` | Explain SQL JOINs | audience-dependent |

## Scoring rubric

| Dimension | Scale | Description |
|---|---|---|
| `clarity` | 0–10 | Easy to understand |
| `completeness` | 0–10 | Fully addresses the task |
| `relevance` | 0–10 | Every sentence is on-topic |
| `engagement` | 0–10 | Interesting to read |
| `total` | 0–40 | Sum of all four |

## Output

| Status | Meaning |
|---|---|
| `winner=A` | Variant A scored higher by more than threshold |
| `winner=B` | Variant B scored higher by more than threshold |
| `winner=tie` | Scores within threshold — side-by-side report committed |
| `status=error` | GenerationError during variant generation |
