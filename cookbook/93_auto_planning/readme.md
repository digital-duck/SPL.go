# Recipe 93 — Automated Planning (PDDL / STRIPS)

**Category:** reasoning · **Tier:** 2 · **Requires:** stdlib `json` only — no new pip dependency

## What this demonstrates

Opens Domain 2 (Automated Planning) from the post-TMLR roadmap: `GENERATE` plan → `CALL` validator → `ASSERT` valid → `WHILE` repair → explain — the two-phase generate-then-verify pattern applied to STRIPS-style planning, the domain where that pattern was born (LLM+P, DUPLEX). Given a fixed logistics-lite domain (a truck, packages, locations; `drive`/`load`/`unload` actions with STRIPS preconditions/effects), the LLM's most common failure mode isn't picking an unreasonable plan — it's silently violating one action's precondition partway through (unloading a package from a truck that isn't there yet) while the plan still *looks* like it reaches the goal. A plan validator ("VAL" in the planning literature) is the crisp, binary oracle: replay the plan action-by-action and the whole trace is either legal, or it isn't.

| Stage | Mode | Tool | Why |
|-------|------|------|-----|
| Parse domain + problem | **Probabilistic** | LLM (`formulate_plan`) | LLMs read the STRIPS domain/problem prose |
| Propose grounded plan | **Probabilistic** | LLM | JSON list of `drive`/`load`/`unload` actions |
| Replay plan step-by-step | **Deterministic** | `run_plan_check()` — mini-VAL validator | Checks EVERY action's precondition against the CURRENT simulated state before applying its effect |
| Check goal | **Deterministic** | `run_plan_check()` | Final state must satisfy `pkg_at` goal predicates |
| Gate on validity | **Deterministic** | `ASSERT is_ok()` | Formal boundary: only continue if the whole plan replayed legally AND reached the goal |
| Repair invalid plan | **Probabilistic** | LLM (`repair_plan`) | LLM sees exactly which step's precondition failed and why |
| Interpret result | **Probabilistic** | LLM (`interpret_plan_result`) | Plain-English explanation of the validated plan |
| Round-trip check | **Deterministic** | `classify_roundtrip()` | Confirms the LLM's own restated step count matches the validated plan length |

**Key property:** VAL's verdict is binary and non-negotiable — a precondition either held at that step in that exact simulated state, or it didn't. This is the same categorical shape as recipe 83's `DimensionalityError` and recipe 91's atom-count mismatch, applied to symbolic action sequences instead of numeric/chemical quantities.

## Setup

No new dependency — the STRIPS domain and plan validator are pure stdlib Python (`json`).

## The fixed domain

```
STRIPS DOMAIN: logistics-lite
Objects: truck t1; packages p1, p2; locations A, B, C
Predicates: truck_at(T, L), pkg_at(P, L), pkg_in(P, T)
Actions:
  drive(T, from, to)   precondition truck_at(T, from)
  load(T, P, L)        precondition truck_at(T, L) AND pkg_at(P, L)
  unload(T, P, L)       precondition truck_at(T, L) AND pkg_in(P, T)
```

## `enable_solver=false` vs `enable_solver=true`

- **`enable_solver=false`** (ARM B, unaided baseline): the LLM plans in prose, reasoning "in its head" about whether each precondition holds at each step. This is exactly where a multi-step plan's silent precondition violation slips by unnoticed.
- **`enable_solver=true`** (ARM A, default): the LLM proposes a grounded JSON action plan; `run_plan_check()` replays it action-by-action against the domain's real STRIPS semantics, checking every precondition in the current simulated state and updating it after each effect; `ASSERT is_ok(@solution)` gates on the full trace being legal AND the goal being reached (repair loop up to `max_tries`, fed the exact failing step and reason); the LLM narrates the validated plan and restates a `Final answer:` step-count line, cross-checked by `classify_roundtrip()`.

## Run

```bash
# Default problem (deliver p1 from A and p2 from B to C; known minimal plan: 6 steps)
spl-go run cookbook/93_auto_planning/auto_planning.spl --llm claude_cli

# Unaided baseline arm
spl-go run cookbook/93_auto_planning/auto_planning.spl \
    --llm ollama:gemma4 --param enable_solver=true
```

To try a different logistics problem, override both `problem_text` (the prose description shown to the LLM) and `problem_json` (the machine-readable initial state + goal the validator replays against) together — they must describe the same scenario.

## Default problem

> Truck t1 starts at A. Package p1 starts at A, package p2 starts at B. Deliver BOTH packages to location C using t1 (t1 can carry any number of packages at once; it can drive directly between any two locations).

**Known minimal plan (6 steps):** load p1@A → drive A→B → load p2@B → drive B→C → unload p1@C → unload p2@C.

Verified end-to-end (2026-07-19) with `--llm claude_cli`: correct 6-step plan on the first attempt, `ASSERT is_ok` passed (goal `{p1: C, p2: C}` reached), round-trip check returned `match`.

## Execution flow

```
GENERATE formulate_plan(@domain, @problem_text)   -- LLM proposes grounded JSON plan
    │
CALL run_plan_check(@problem_json, @plan)         -- mini-VAL: replay step-by-step
    │
WHILE @tries < @max_tries                         -- repair loop on precondition violation
    │
ASSERT is_ok(@solution)                           -- hard gate: AssertionError if not OK
    │
GENERATE interpret_plan_result(...)                -- LLM explains + states Final answer
    │
CALL classify_roundtrip(@narrative, @solution)     -- LLM's stated step count vs validated
    │
CALL format_report(...)                           -- Markdown report
```

## Exception handling

If no VAL-valid plan is found within `max_tries`, `ASSERT is_ok` raises `AssertionError`, caught by `EXCEPTION WHEN ToolFailed THEN`. The workflow exits with `status = "error"` and `roundtrip = "unverifiable"` rather than ever returning an unvalidated plan.

## Why this recipe next

This single workflow generalizes both LLM+P and DUPLEX (currently Related Work citations in the TMLR paper) into a head-to-head baseline — the non-SPL-baseline comparison reviewers asked for, in the domain where the generate-then-verify two-phase pattern was born. Highest academic payoff per unit effort: comparisons become apples-to-apples against published planning numbers once extended to real PDDL/IPC domains and an external VAL binary.
