# Recipe 16: Reflection Agent

Meta-cognitive loop: the agent solves a problem, reflects on its own reasoning to find errors, corrects them, and re-reflects until confidence exceeds 0.85 or the iteration limit is reached.

## Pattern

```
solve(problem) → @answer
  └─► WHILE iteration < 3:
        reflect(problem, answer) → @reflection
        confidence_score(answer, reflection) → @confidence
          ├─ > 0.85 → COMMIT confident
          └─ else   → extract_issues → correct → iterate
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `problem` | TEXT | *(required)* | The question or design problem to solve |

## Usage

```bash
spl-go run cookbook/16_reflection/reflection.spl --adapter ollama \
    problem="What are the trade-offs of microservices vs monoliths?"

spl-go run cookbook/16_reflection/reflection.spl --adapter ollama \
    problem="Design a URL shortener system"

spl-go run cookbook/16_reflection/reflection.spl --adapter ollama --model llama3.2 \
    problem="Explain why bubble sort is O(n^2)"
```

## Output status

| Status | Meaning |
|---|---|
| `confident` | Confidence > 0.85 before limit |
| `best_effort` | Loop exhausted; best answer committed |
| `max_reflections` | Exception handler triggered |
| `restarted` | Hallucination detected; re-solved from scratch |
