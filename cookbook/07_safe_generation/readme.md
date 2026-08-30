# Recipe 07: Safe Generation

LLM-as-judge quality pipeline with exception handling. Generates a response, assesses quality, and routes to different commit statuses based on the verdict.

## Pattern

```
response(prompt)
  └─► quality_assess(result)
        ├─ high_quality  → COMMIT as-is
        ├─ acceptable    → improved(result, prompt) → COMMIT refined
        └─ otherwise     → COMMIT best_effort
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `prompt` | TEXT | *(required)* | The input prompt to generate a response for |

## Usage

```bash
spl-go run cookbook/07_safe_generation/safe_generation.spl --adapter ollama \
    --model phi4 prompt="Explain how internet works" \
    2>&1 | tee cookbook/out/07_safe_generation-$(date +%Y%m%d_%H%M%S).md 

spl-go run cookbook/07_safe_generation/safe_generation.spl --adapter ollama \
    prompt="What are the trade-offs of microservices?"

spl-go run cookbook/07_safe_generation/safe_generation.spl --adapter claude_cli \
    prompt="Summarise the history of the internet"
```

## Output status

| Status | Meaning |
|---|---|
| `high_quality` | Quality judge approved on first pass |
| `refined` | Improved after acceptable-quality pass |
| `best_effort` | Quality was low; committed anyway |
| `conservative` | Re-generated after hallucination detected |
| `refused` | Model refused to answer |
| `truncated` | Token budget exceeded |
