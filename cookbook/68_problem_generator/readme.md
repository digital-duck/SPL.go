# Recipe 68 — Answer-First Problem Generator

**Pattern:** `CREATE TOOL_API` (deterministic SymPy) generates the answer *first*;
`CREATE FUNCTION` (LLM) writes only the narrative around it

## What this demonstrates

The key inversion: **SymPy generates the exact answer before the LLM ever sees the
problem**. The LLM then writes the question and the worked solution *around* that
guaranteed-correct answer. It never touches the math — only the words. Hallucination
on the computation is structurally impossible, because the computation already
happened, exactly, before any LLM call.

Flow per problem:

```
Deterministic (SymPy)  — generate expression + exact answer
Probabilistic  (LLM)   — write an engaging word problem (never sees the derivation)
Probabilistic  (LLM)   — write a step-by-step worked solution ending in that exact answer
```

Output is a complete, print-ready exercise set in Markdown, with collapsible
`<details>` solutions.

## Architecture

```
GENERATE write_lesson_intro()   → probabilistic: one-paragraph lesson intro
WHILE @n IN @numbers DO
    CALL generate_math_problem() → deterministic: SymPy expr + exact_answer (no LLM)
    GENERATE write_word_problem() → probabilistic: 2-sentence exercise (answer hidden)
    GENERATE write_worked_solution() → probabilistic: numbered steps ending in exact_answer
END
```

## Supported parameters

| Parameter      | Values                                                              |
|----------------|---------------------------------------------------------------------|
| `@topic`       | `quadratic equations`, `derivatives`, `integration`, `factoring`, `linear equations` |
| `@difficulty`  | `easy`, `medium`, `hard`                                            |
| `@numbers`     | comma-separated problem indices, e.g. `'1,2,3'` → 3 problems        |

## Prerequisites

```bash
pip install sympy
```

(or run with `--self-healing` for automatic install)

## Run

```bash
# Default: 3 medium quadratic-equation problems
spl-go run cookbook/68_problem_generator/problem_generator.spl \
    --llm claude_cli

# Custom topic, difficulty, and count
spl-go run cookbook/68_problem_generator/problem_generator.spl \
    --llm claude_cli \
    --param topic="derivatives" \
    --param difficulty="hard" \
    --param numbers="1,2,3,4,5"
```

## Key learning points

1. **Answer-first generation eliminates an entire class of bugs.** A traditional
   "LLM writes a problem, then solves it" pipeline can produce a question whose
   stated answer is wrong. Here the answer is fixed in stone — by SymPy — before
   the question is even drafted.

2. **The LLM is *structurally* prevented from revealing the answer early.**
   `write_word_problem` is given the raw SymPy result but instructed never to
   reveal it; `write_worked_solution` is told to treat that result as ground truth
   and must end on the unchanged `exact_answer`.

3. **One `TOOL_API`, five problem types.** `generate_math_problem` dispatches on
   the `@topic` string (`quadratic`, `deriv`/`differ`, `integr`/`antid`, `factor`,
   `linear`) and on `@difficulty` (`easy` / `medium` / `hard` controls the number
   pools used for random generation), all inside one Python tool body.

4. **`WHILE @n IN @numbers`** drives the answer-first loop once per requested
   problem, accumulating Markdown sections into `@lesson` with collapsible
   `<details><summary>Worked Solution</summary>` blocks.
