# Recipe 04 — Model Showdown

Same prompt to multiple Ollama models — compare output quality and latency side-by-side.

Uses parallel CTEs to fan out the prompt across models in a single SPL workflow, then generates a comparative evaluation.

## Usage

```bash
spl-go run cookbook/04_model_showdown/showdown.spl --adapter ollama \
    --param prompt="What is the benefit of meditation?"

```

Override the models:
```bash
spl-go run cookbook/04_model_showdown/showdown.spl --adapter ollama --model gemma4:e2b \
    --param prompt="Write a poem about Spring season" \
    --param model_1=gemma3 \
    --param model_2=phi3
```

## Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `prompt`  | `What is the benefit of meditation?` | The question sent to all models |
| `model_1` | `gemma3`   | First model  |
| `model_2` | `llama3` | Second model |


## How it works

Three CTEs fan out the same prompt to each model in parallel. The results are collected into `@answer_1/2/3`, then a judge step synthesizes a side-by-side comparison highlighting strengths and differences.

This is the SPL 3.0 equivalent of Momahub's Recipe 08 (Model Arena) — but runs locally with zero infrastructure.
