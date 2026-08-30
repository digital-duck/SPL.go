# Recipe 05: Self-Refine

Iteratively improves an LLM output through a critique-and-refine loop.
Each iteration critiques the current draft; if the critique judges it
satisfactory the loop commits early, otherwise it generates a refined version
and continues. The loop is capped by `@max_iterations` (default 5).

## Pattern

```
draft(task)
  └─► critique(current)
        ├─ satisfactory → COMMIT (early exit)
        └─ needs work  → refined(current, feedback) → next iteration
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `task` | TEXT | *(required)* | The writing or generation task |
| `max_iterations` | INT | `5` | Maximum refinement cycles before committing best effort |

## Usage

### Minimal — use all defaults
```bash
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter ollama \
    task="Write a haiku about coding"
```

### Custom iteration limit
```bash
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter ollama \
    task="Write a haiku about coding" \
    max_iterations=3
```

### With a different model
```bash
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter ollama --model llama3.2 \
    task="Explain recursion in one paragraph" \
    max_iterations=4
```

### Via Claude Code CLI
```bash

spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter ollama \
    task="Write an executive summary of the benefits of SPL - Structured Prompt Language" \
    max_iterations=3

spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter claude_cli \
    task="Who is Srinivasa Ramanujan" \
    max_iterations=3 \
    output_budget=3000 \
    writer_model=claude-sonnet-4-6 \
    critic_model=claude-sonnet-4-6 \
    log_dir=cookbook/05_self_refine/logs-ramanujan 

export OPENAI_API_KEY=sk-...
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter openai --model gpt-4o-mini\
    task="How does Transformer work in deep learning" \
    max_iterations=3 \
    output_budget=2000 \
    log_dir=cookbook/05_self_refine/logs-openai

export ANTHROPIC_API_KEY=sk-...
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter anthropic --model claude-sonnet-4-5-20250929 \
    task="How does Transformer work in deep learning" \
    max_iterations=3 \
    output_budget=2000 \
    log_dir=cookbook/05_self_refine/logs-anthropic


export OPENROUTER_API_KEY=sk-...
spl-go run cookbook/05_self_refine/self_refine.spl \
    --adapter openrouter \
    task="How does Transformer work in deep learning" \
    max_iterations=3 \
    output_budget=3000 \
    writer_model=anthropic/claude-sonnet-4.6 \
    critic_model=anthropic/claude-haiku-4.5 \
    log_dir=cookbook/05_self_refine/logs-openrouter-3

```

## Output status

| Status | Meaning |
|---|---|
| `complete` | Critique judged output satisfactory before hitting the limit |
| `max_iterations` | Loop ran to completion; best effort committed |
| `partial` | `MaxIterationsReached` exception caught |
| `budget_limit` | Token budget exceeded during refinement |
