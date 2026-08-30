# Recipe 25 — Nested Procedures

`PROCEDURE` calling `PROCEDURE` — deep composability without external orchestration.

## Key Feature

Each `PROCEDURE` is a reusable unit with typed `INPUT`/`OUTPUT`. The outer `WORKFLOW` composes them via `CALL`, and inner procedures can themselves call others. No bash glue required.

## Structure

```
WORKFLOW layered_explainer
  └── CALL explain_layer(...)       # inner PROCEDURE
  └── CALL make_example(...)        # inner PROCEDURE
  └── CALL calibrate_complexity(...)  # inner PROCEDURE (calls GENERATE internally)
```

## Usage

```bash
# Explain quantum computing to high schoolers
spl-go run cookbook/25_nested_procs/nested_procs.spl --adapter ollama --model gemma3 \
    topic="quantum computing" audience="high school students"

# Deep-dive for policy makers
spl-go run cookbook/25_nested_procs/nested_procs.spl --adapter ollama \
    topic="CRISPR gene editing" audience="policy makers" depth="deep"

# Simple intro for a general audience
spl-go run cookbook/25_nested_procs/nested_procs.spl --adapter ollama \
    topic="machine learning" audience="non-technical managers" depth="intro"
```

## Why nested procedures

- Each `PROCEDURE` is independently testable
- Composing them in `WORKFLOW` reads like a specification — not implementation
- Adding a new layer (e.g., `add_quiz_questions`) requires one new `PROCEDURE` + one new `CALL` line
