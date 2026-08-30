# Recipe 17: Tree of Thought

Explores three independent reasoning paths in parallel, scores each, selects the best, refines it, and verifies the result. Falls back to synthesizing all paths if verification fails.

## Pattern

```
approach_1/2/3(problem)
  └─► develop each path
        └─► evaluate_path (score A/B/C)
              └─► select_best → refine_solution → verify
                    ├─ sound      → COMMIT complete
                    └─ unsound    → synthesize_all → COMMIT synthesized
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `problem` | TEXT | *(required)* | The problem or decision to explore |

## Usage

```bash
spl-go run cookbook/17_tree_of_thought/tree_of_thought.spl --adapter ollama \
    problem="Should we rewrite the legacy system or incrementally refactor?"

spl-go run cookbook/17_tree_of_thought/tree_of_thought.spl --adapter ollama \
    problem="Design a caching strategy for a high-traffic API"

spl-go run cookbook/17_tree_of_thought/tree_of_thought.spl --adapter ollama --model llama3.2 \
    problem="How should a startup prioritize between growth and profitability?"
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | Best path verified as sound |
| `synthesized` | Verification failed; all paths merged |
| `budget_limit` | Budget exceeded; best scored path returned |
