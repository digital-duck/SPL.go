# Recipe 56: Code Pipeline

**Category:** workflow composition  
**SPL version:** 3.0  
**Demonstrates:** `CALL` sub-workflow dispatch, `WHILE` + `EVALUATE` test-gated retry, spec closure check, `EXCEPTION WHEN` error handling

---

## What it does

A full autonomous code generation lifecycle with test-gated iteration and
a **closure check** — a fixed-point verification that the implementation
faithfully captured the original intent.

```
Step 1: Generate → Review → Improve  ←────────────────────────────┐
Step 2: Test                                                        │
   [PASSED] → Step 3                                               │
   [FAILED]  → back to Step 1 (up to max_cycles) ─────────────────┘

Step 3a: Document    — human-readable docs (README / user guide)
Step 3b: ExtractSpec — reverse-engineer a spec from the final code
Step 3c: SpecJudge   — LLM-as-judge: does extract_spec(code) ≈ original spec?
   [CLOSED]   → implementation matches intent  ✓
   [DIVERGED] → semantic drift detected        ⚠
```

### Closure principle

The closure check enforces a **fixed-point condition** on autonomous vibe coding:

> If `extract_spec(generate(spec)) ≈ spec`, the system has reached a
> self-consistent state — the code does what was originally asked.
>
> Divergence reveals semantic drift: the code does *something*, but not
> what was intended.

This is analogous to a round-trip serialisation test: encode → decode → compare.
When the round-trip holds, you have mathematical confidence in correctness of intent.

---

## Files

| File | Step | Role |
|------|------|------|
| `code_pipeline.spl` | Orchestrator | Implements the full 3-step lifecycle |
| `generate_code.spl` | 1a | Generate Python code from spec |
| `review_code.spl` | 1b | Identify correctness issues and edge cases |
| `improve_code.spl` | 1c | Rewrite code addressing review feedback |
| `test_code.spl` | 2 | Evaluate code against spec; outputs `[PASSED]` / `[FAILED]` |
| `document_code.spl` | 3a | Generate Markdown documentation |
| `extract_spec.spl` | 3b | Reverse-engineer a formal spec from the final code |
| `spec_judge.spl` | 3c | LLM-as-judge comparing original vs derived spec |

---

## Lifecycle detail

### Step 1: Generate → Review → Improve

One pass per cycle:

1. `generate_code(@spec)` — writes initial Python from the natural-language spec
2. `review_code(@code)` — returns a bullet list of specific correctness issues
3. `improve_code(@code, @feedback)` — rewrites the code addressing all feedback

### Step 2: Test

`test_code(@code, @spec)` has an LLM act as an automated test suite,
checking the code satisfies the specification.

- `[PASSED]` → proceed to Step 3
- `[FAILED]` → increment cycle counter, restart from Step 1
- After `@max_cycles` failed cycles, the pipeline proceeds with the best-effort code

### Step 3a: Document

`document_code(@code, @spec)` generates a Markdown reference covering:
overview, function/class reference, edge cases, and a usage example.

### Step 3b: Extract spec

`extract_spec(@code)` reverse-engineers a formal specification from the
final implementation — describing *what* the code does (not *how*), in the
same natural-language style as the original spec.

### Step 3c: Closure check (optional, default on)

`spec_judge(@spec, @out_spec)` compares the original spec against the
derived spec across four dimensions:

| Dimension | What is checked |
|-----------|----------------|
| Purpose alignment | Do they describe the same problem? |
| Input/output contract | Do signatures and types agree? |
| Behavioural completeness | Does the derived spec cover all cases in the original? |
| Semantic drift | Does the derived spec introduce behaviour not in the original? |

The closure report is appended to `@docs` as a final section.

---

## Running

### Prerequisites

- Ollama running locally with `gemma4` pulled:
  ```bash
  ollama pull gemma4
  ```
- SPL runtime installed (`spl` CLI available)

### Basic run (closure check enabled by default)

```bash
spl-go run cookbook/50_code_pipeline/code_pipeline.spl \
    --adapter ollama \
    --param pipeline_model=gemma3 \
    --param spec="Write a binary search function that returns the index or -1" \
    --param check_closure=FALSE


```

### Skip the closure check

```bash
spl-go run cookbook/50_code_pipeline/code_pipeline.spl \
    --param spec="Write a binary search function" \
    --param check_closure=FALSE
```

### Custom model and cycle limit

```bash
spl-go run cookbook/50_code_pipeline/code_pipeline.spl \
    --param spec="Write a function to parse ISO 8601 dates" \
    --param model="llama3.2" \
    --param max_cycles=5
```

### Run on Momagrid (Hub dispatch)

```bash
spl-go run cookbook/50_code_pipeline/code_pipeline.spl \
    --adapter momagrid \
    --hub http://localhost:8080 \
    --param spec="Write a function to flatten a nested list"
```

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@spec` | TEXT | *(required)* | Natural-language specification of the code to generate |
| `@model` | TEXT | `gemma4` | LLM model used across all sub-workflows |
| `@max_cycles` | INTEGER | `3` | Maximum generate→test retry cycles before proceeding |
| `@check_closure` | BOOL | `TRUE` | Enable spec closure check in Step 3 |
| `@log_dir` | TEXT | `cookbook/50_code_pipeline/logs-spl` | Directory for log output |

---

## Output

`@docs TEXT` — Markdown document containing:

1. **Code documentation** — overview, function reference, edge cases, usage example
2. **Closure Report** *(if `@check_closure = TRUE`)* — original spec, derived spec, and the judge's verdict (`[CLOSED]` or `[DIVERGED]` with analysis)

---

## Expected log output

```
[code_pipeline] started | spec="..." max_cycles=3 check_closure=True
[code_pipeline] cycle=1 | step 1: generate
[generate_code] started | spec="..."
[generate_code] done | output_len=512
[code_pipeline] cycle=1 | step 1: review
[review_code] started
[review_code] done | feedback_len=310
[code_pipeline] cycle=1 | step 1: improve
[improve_code] started
[improve_code] done | output_len=530
[code_pipeline] cycle=1 | step 2: test
[test_code] started
[test_code] done | result=[PASSED]
[code_pipeline] tests passed at cycle=1
[code_pipeline] step 3: document
[document_code] started
[document_code] done | docs_len=1024
[code_pipeline] step 3: extract spec from implementation
[extract_spec] reverse-engineering spec from implementation ...
[extract_spec] done | out_spec_len=480
[code_pipeline] step 3: closure check — spec vs derived spec
[spec_judge] comparing original spec vs derived spec ...
[spec_judge] verdict: CLOSED — implementation matches intent
[code_pipeline] done | cycles=1 test_passed=True check_closure=True
```

---

## Error handling

| Exception | Where raised | Behaviour |
|-----------|-------------|-----------|
| `RefusalToAnswer` | `generate_code` | Pipeline exits immediately with `status = 'refused'` |
| `ModelUnavailable` | any sub-workflow | Sub-workflow returns error string; orchestrator catches at top level |

---

## Interpreting closure results

| Verdict | Meaning | Suggested action |
|---------|---------|-----------------|
| `[CLOSED]` | Implementation faithfully captures the original intent | Ship it |
| `[DIVERGED]` | Semantic drift — code does something, but not exactly what was asked | Review the diff in the closure report; refine `@spec` and rerun |

A `[DIVERGED]` result is not necessarily a failure — it may reveal that the
original spec was ambiguous. The divergence analysis is itself valuable
documentation of what the LLM inferred vs. what was intended.
