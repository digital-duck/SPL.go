# Recipe 91 — Chemical Equation Balancing

**Category:** reasoning · **Tier:** 2 · **Requires:** stdlib `re` only — no new pip dependency

## What this demonstrates

Second recipe in the computational-science domain (companion to `90_compsci_physics` and `92_compsci_materials`). Balancing a chemical equation is a classic LLM near-miss: the proposed coefficients *look* plausible — right formulas, small integers — but silently fail to conserve mass for at least one element. Atom-counting from the chemical formulas alone is a categorical oracle: no chemistry library (RDKit, etc.) is required, just a formula parser and elemental bookkeeping.

| Stage | Mode | Tool | Why |
|-------|------|------|-----|
| Parse natural-language reaction | **Probabilistic** | LLM (`formulate_balance`) | LLMs read prose reaction descriptions |
| Propose balanced equation | **Probabilistic** | LLM | Explicit integer coefficients, strict format |
| Parse formulas (incl. parentheses) | **Deterministic** | `run_balance_check()` — recursive-descent formula parser | Turns `Ca(OH)2` into `{Ca:1, O:2, H:2}` etc. |
| Check mass balance | **Deterministic** | `run_balance_check()` | Sums atoms x coefficient per element on each side; must match exactly |
| Gate on balance | **Deterministic** | `ASSERT is_ok()` | Formal boundary: only continue if every element balances exactly |
| Repair failed proposal | **Probabilistic** | LLM (`repair_balance`) | LLM sees the actual mismatched element counts |
| Interpret result | **Probabilistic** | LLM (`interpret_balance_result`) | Plain-English explanation of the verified equation |
| Round-trip check | **Deterministic** | `classify_roundtrip()` | Confirms the LLM's own restated coefficient sequence matches the verified balance |

**Key property:** an equation either conserves every element's atom count exactly, or it doesn't — there is no partial credit, making this a Pattern-2/3-style categorical gate rather than a numeric-fuzzy one (per Appendix I's verification-strength taxonomy).

## Setup

No new dependency — the formula parser and balance checker are pure stdlib Python (`re` only).

## `enable_solver=false` vs `enable_solver=true`

- **`enable_solver=false`** (ARM B, unaided baseline): the LLM balances the equation in prose, tracking atom counts "in its head." This is exactly where models propose a nearly-right coefficient set that's off by one atom on some element.
- **`enable_solver=true`** (ARM A, default): the LLM proposes a strict-format equation string (`<coeff> <Formula> + ... -> <coeff> <Formula> + ...`); `run_balance_check()` parses every formula and checks exact atom-count equality on both sides; `ASSERT is_ok(@solution)` gates on exact mass balance (repair loop up to `max_tries`, fed the real mismatched elements); the LLM narrates the verified equation and restates a `Final answer:` coefficient line, cross-checked by `classify_roundtrip()`.

## Run

```bash
# Default problem (combustion of methane: 1 CH4 + 2 O2 -> 1 CO2 + 2 H2O)
spl-go run cookbook/91_compsci_chemistry/chemistry_balance.spl --llm claude_cli

# Custom problem (parenthesized formula)
spl-go run cookbook/91_compsci_chemistry/chemistry_balance.spl \
    --llm ollama:gemma3 \
    --param problem="Neutralization: Ca(OH)2 + HCl -> CaCl2 + H2O"

# Unaided baseline arm
spl-go run cookbook/91_compsci_chemistry/chemistry_balance.spl \
    --llm claude_cli --param enable_solver=false
```

## Default problem

> Combustion of methane: CH4 + O2 -> CO2 + H2O

**Known closed-form answer:** `1 CH4 + 2 O2 -> 1 CO2 + 2 H2O` (C: 1=1, H: 4=4, O: 4=4).

Verified end-to-end (2026-07-19) with `--llm claude_cli`: correct balanced equation on the first attempt, `ASSERT is_ok` passed, round-trip check returned `match`.

## Execution flow

```
GENERATE formulate_balance(@problem)          -- LLM proposes coefficients
    │
CALL run_balance_check(@equation)             -- parse formulas, sum atoms per element
    │
WHILE @tries < @max_tries                     -- repair loop on mass-balance mismatch
    │
ASSERT is_ok(@solution)                       -- hard gate: AssertionError if not OK
    │
GENERATE interpret_balance_result(...)         -- LLM explains + states Final answer
    │
CALL classify_roundtrip(@narrative, @solution) -- LLM's stated coefficients vs verified
    │
CALL format_report(...)                       -- Markdown report
```

## Exception handling

If the equation cannot be reconciled to an exact mass balance within `max_tries`, `ASSERT is_ok` raises `AssertionError`, caught by `EXCEPTION WHEN ToolFailed THEN`. The workflow exits with `status = "error"` and `roundtrip = "unverifiable"` rather than ever returning an unbalanced equation.

## Why this recipe next

Continues Domain 1 (Computational Science) from the post-TMLR roadmap: "reaction balancing, valence checks" is the oracle table's chemistry row — implemented here without a chemistry-specific library, keeping the recipe's only dependency at stdlib `re`, matching the zero-new-dependency bar set by `84_sql_verifier`.
