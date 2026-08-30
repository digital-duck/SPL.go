# Recipe 21: Multi-Model Pipeline

Per-step model selection with `GENERATE ... USING MODEL`. Each stage targets the model best suited for that task (factual retrieval, analysis, writing), followed by a quality loop that refines until score > 0.7.

## Pattern

```
research(@topic)    USING MODEL gemma3   → @facts
analyze(@facts)     USING MODEL gemma3   → @analysis
write_summary(@analysis) USING MODEL gemma3 → @draft
  └─► WHILE iteration < 3:
        quality_check(@draft) → @quality
          ├─ > 0.7 → COMMIT high_quality
          └─ else  → rewrite → iterate
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `topic` | TEXT | *(required)* | The topic to research, analyze, and write about |

## Usage

```bash
spl-go run cookbook/21_multi_model_pipeline/multi_model.spl --adapter ollama \
    topic="climate change"

spl-go run cookbook/21_multi_model_pipeline/multi_model.spl --adapter ollama \
    topic="the future of edge computing"
```

## Notes

- Model names in the `.spl` file are hardcoded to `gemma3`. Edit the `USING MODEL` clauses to route to different models per step.
- The quality loop runs a maximum of 3 iterations before committing best effort.

## Output status

| Status | Meaning |
|---|---|
| `high_quality` | Score > 0.7 before iteration limit |
| `max_iterations` | Loop exhausted; best draft committed |
| `partial` | MaxIterationsReached exception |
| `model_overloaded` | Target model unavailable |
