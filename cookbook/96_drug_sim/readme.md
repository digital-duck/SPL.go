# Recipe 96 — Pharmacovigilance: Drug-Interaction Screening

**Category:** reasoning · **Tier:** 2 · **Requires:** stdlib `json`/`re` only — no new pip dependency

> **SYNTHETIC DEMO ONLY.** The interaction table is a small, illustrative set of well-known textbook drug pairs, NOT a complete or clinical-grade database. This is a research prototype demonstrating SPL's two-mode architecture, not clinical decision support, and must not inform real medication decisions. A real deployment would register RxNorm (name normalization) and DrugBank/OpenFDA (interaction lookup) as the `CREATE TOOL_API` backends instead of the seeded dict here.

## What this demonstrates

Suggested by Qwen (2026-07-25, see `docs/publication/TMLR/SPL-for-Agentic-Workflow/review-feedback/LLM/qwen-3-8-max-full-20260725.md`) as the domain where the two-mode split is **not optional**: a clinician's free-text note must be read by an LLM (probabilistic — "the usual diabetes medication" vs "metformin 500mg BID"), but the interaction check cannot be probabilistic — a hallucinated "no interaction found" is a patient-safety event, not a wrong answer.

| Stage | Mode | Tool | Why |
|-------|------|------|-----|
| Extract medications from prose | **Probabilistic** | LLM (`extract_medications`) | LLMs read free-text clinical notes; a lookup table doesn't |
| Normalize + screen | **Deterministic** | `screen_regimen()` | Synthetic RxNorm-style alias table + synthetic DrugBank-style pairwise severity table |
| Safety gate (load-bearing) | **Deterministic** | inline `EVALUATE`/`ASSERT` | A `contraindicated` pair halts BEFORE narration, unconditionally |
| Repair loop | **Probabilistic + Deterministic** | `WHILE` + `suggest_alternative` | For `moderate` pairs only: LLM proposes a substitution, kernel re-screens |
| Extraction round-trip | **Deterministic** | `classify_roundtrip()` | Deterministic keyword recount of the note vs. the LLM's extracted drug count — catches silently dropped/invented medications |
| Interpret result | **Probabilistic** | LLM (`explain_regimen` / `draft_alert`) | Plain-English summary or urgent clinical alert |

**Key property:** the `ASSERT` gate is load-bearing in the sense Qwen originally described — no amount of prompt engineering makes an LLM-only version safe here, because there is no path from extraction straight to narration that skips the deterministic severity check.

## Seeded synthetic reference

```
Drugs:  sildenafil, nitroglycerin, warfarin, aspirin, ibuprofen,
        simvastatin, clarithromycin, lisinopril, spironolactone,
        metformin, acetaminophen  (+ common brand-name aliases)

Interactions (severity, illustrative only):
  sildenafil + nitroglycerin     -> contraindicated (life-threatening hypotension)
  warfarin + aspirin             -> severe (additive bleeding risk)
  simvastatin + clarithromycin   -> severe (rhabdomyolysis risk)
  warfarin + ibuprofen           -> moderate (GI bleeding risk)
  lisinopril + spironolactone    -> moderate (hyperkalemia risk)
```

## `enable_solver=false` vs `enable_solver=true`

- **`enable_solver=false`** (ARM B, unaided baseline): the LLM screens from memory alone, no interaction database consulted — exactly the failure mode this recipe exists to prevent.
- **`enable_solver=true`** (ARM A, default): LLM extracts a structured drug list; `screen_regimen()` normalizes and checks every pairwise combination; a `contraindicated` result halts immediately with a drafted alert; a `moderate` result enters a bounded repair loop (LLM suggests dropping/substituting one drug, kernel re-screens); `ASSERT` gates on nothing severe/contraindicated remaining; the LLM narrates the cleared regimen.

## Run

```bash
# Default: a well-known contraindicated pair (sildenafil + nitroglycerin)
spl-go run cookbook/96_drug_sim/drug_sim.spl --llm claude_cli

# Moderate-severity repair-loop path
spl-go run cookbook/96_drug_sim/drug_sim.spl --llm ollama:gemma4 \
    --param clinical_note="Patient is on lisinopril 10mg daily for hypertension and spironolactone 25mg daily for heart failure."

# Unaided baseline arm
spl-go run cookbook/96_drug_sim/drug_sim.spl --llm claude_cli --param enable_solver=false
```

## Verified (2026-07-26, `--llm ollama:gemma4`)

- Default contraindicated case: extraction correct (sildenafil + nitroglycerin), severity=`contraindicated`, halted before narration, alert drafted, report written.
- Moderate case (lisinopril + spironolactone): repair loop fired once, LLM dropped spironolactone, re-screen returned `none`, `ASSERT` passed, round-trip=`match`.

## Execution flow

```
GENERATE extract_medications(@reference, @clinical_note)   -- LLM extracts drug list
    │
CALL screen_regimen(@drug_list)                             -- normalize + pairwise severity lookup
    │
EVALUATE severity == "contraindicated"?
    │  YES -> draft_alert(), RETURN status=contraindication_found  (halts here, unconditionally)
    │  NO  ↓
WHILE severity == "moderate" AND @iter < @max_tries          -- bounded repair loop
    │      suggest_alternative() -> screen_regimen()
    ↓
ASSERT severity NOT IN (severe, contraindicated)             -- hard gate
    │
CALL classify_roundtrip(@clinical_note, @solution)           -- extraction-fidelity check
    │
GENERATE explain_regimen(...)                                -- LLM narrates verified result
    │
CALL format_report(...)                                      -- Markdown report
```

## Why a synthetic dataset, not RxNorm/DrugBank

Consulting a domain expert before wiring in a real clinical database is the right order of operations. The seeded dict here proves the *architecture* — extraction, pairwise screening, load-bearing ASSERT, bounded repair — without any licensing, API-key, or regulatory surface area. Swapping in a real oracle is a `CREATE TOOL_API` implementation change only; the `.spl` workflow does not need to change.
