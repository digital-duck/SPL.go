# Recipe 71 — Linear Algebra Concept-Book Generator

**Pattern:** domain concept graph (`linalg_graph.py`) + style profiles
(`style_profiles.py`) + `SOLVE` / `ASSERT` (deterministic) + `GENERATE` /
`refine_section` (probabilistic) — compiled to a notebook via the
`python/linalg` `splc` target (**DODA**: logical `.spl` view → physical
notebook artifact)

## What this demonstrates

A complete **neurosymbolic authoring pipeline** for a linear-algebra
concept-book, where a pre-built concept graph drives *what* gets taught and
in *what order* (deterministically), while the LLM only ever writes and revises
prose *within* the structure the graph guarantees is sound:

```
Deterministic (graph)   — verify the concept graph is acyclic and reducible to primitives
Deterministic (graph)   — compute the teaching order (topological + payoff-weighted reach)
Probabilistic  (LLM)    — write each section, in a chosen prose style
Deterministic (checks)  — verify worked examples (SymPy) and matrix shapes; refine on failure
Probabilistic  (LLM)    — write the capstone "Payoff" section tying the target to applications
```

This is **Layer 4** of the neurosymbolic-SPL roadmap: a domain library
(`linalg_graph.py`, Layer 2) plus two `WORKFLOW`s (Layer 4a/4b) that consume it,
compiled to a runnable artifact by the `python/linalg` `splc` transpiler (Layer 3) —
plus `lean_payoffs.spl`, a proof-grade Lean post-pass over the build run's
payoff concepts (verifier ladder B-2, run separately).

## The concept graph (`linalg_graph.py`)

A `networkx.DiGraph` of **37 nodes / 51 edges** encoding linear algebra as a
composition hierarchy, in the spirit of dimensional analysis:

- **5 primitives** (tier 0, the irreducible "radicals"): `field_of_scalars`,
  `carrier_set`, `vector_addition`, `scalar_multiplication`, `inner_product`.
  Two independent radicals — `⊕`/`⊙` (algebra) and `⟨·,·⟩` (geometry) — meet at
  `spectral_theorem`.
- **27 concepts** (tiers 1–6), each declaring its `composed_of` prerequisites,
  e.g. `eigenpair = composed_of[linear_map, scalar_multiplication]`,
  `spectral_theorem = composed_of[adjoint, eigenpair, orthonormal_basis]`.
- **7 applications** (the payoff, pointing *out* of linear algebra):
  `pca`, `pagerank`, `markov_stationary`, `normal_modes`, `stability_analysis`,
  `quantum_observable`, `fourier_modes`.

Edges `u → v` mean "u is a prerequisite of v". Key deterministic operations:

| Function | Purpose |
|---|---|
| `acyclic(graph)` | the composition hierarchy has no cycles |
| `reducible(graph, primitives)` | every concept reduces transitively to a declared primitive (no undeclared "floor") |
| `ancestors(graph, target)` / `restrict` | scope the graph to exactly what `target` needs |
| `productivity_order(graph, weight)` | topological order, tie-broken by **payoff-weighted reach** — concepts that unlock the most downstream concepts *and* applications are taught earliest |
| `gap(graph, target, learner_state)` / `learning_path(...)` | ancestors minus what the learner already knows, in productivity order — the personalised slice |
| `applications_of(graph, target)` | the external domains a concept unlocks (feeds the capstone "Payoff" section) |
| `new_primitives(section)` | counts primitive names mentioned in generated prose, to enforce the "introduce ≤ N radicals per section" budget |

## Style profiles (`style_profiles.py`)

`@style` selects a **prose** profile — tone, depth, audience, length, structure —
without changing *what* is taught or *whether* it's correct. `style_instruction(@style)`
is resolved once via `SOLVE` and injected into every `GENERATE` prompt as
`{style_guide}`. The symbolic checks (`verify_math`, `shape_check`, `reducible`)
are completely **style-agnostic** — only the writing adapts.

| Style | Audience | Structure |
|---|---|---|
| `textbook` | first-year university student | Definition → Worked example → Key theorem → Lab cell |
| `feynman` | curious person, no university math | Motivating story → Intuition → Minimal formalisation → "Now you try" |
| `flashcard` | student cramming the night before | Q: ... A: ... Example: ... (one fact per card) |
| `instructor` | instructor prepping a lecture | Concept summary → Common mistakes → Teaching tip → Exercise |
| `research` | grad student / researcher | Definition → Theorem → Proof → Remark (citation-ready) |

## Three workflows

### `build_concept_book.spl` — full curriculum toward a target

```spl
SOLVE @style_guide := style_instruction(@style)             -- resolve prose style once
ASSERT acyclic(@graph)                                       -- graph sanity (deterministic)
ASSERT reducible(@graph, @primitives)
SOLVE @needed   := ancestors(@graph, @target)                -- scope to what @target needs
SOLVE @order    := productivity_order(restrict(@graph, @needed), weight=@payoff_weight)
WHILE @_i < len(@order) DO
    SOLVE @t_section_start := _now()                          -- timing: per-section wall clock
    GENERATE write_section(@concept, @concept, @style_guide)
    ASSERT new_primitives(@section) <= @primitive_budget
        OTHERWISE GENERATE refine_section(...)               -- enforce ≤ N new radicals/section
    CALL verify_math(@section)    → refine on "fail"         -- SymPy recompute of worked examples
    CALL shape_check(@section)    → refine on "fail"         -- matrix-dimension check
    LOGGING f"[timing] {@concept}: {_elapsed(@t_section_start):.1f}s"
END
GENERATE write_payoff(@target, applications_of(@graph, @target), @style_guide)  -- capstone
LOGGING f"[timing] workflow total: {_elapsed(@t_workflow_start):.1f}s"
```

Builds a complete, dependency-ordered textbook from the primitives up to
`@target` (default: `spectral_theorem`), ending in a "Payoff" section that ties
the target concept to the real-world domains it unlocks.

### `answer_on_demand.spl` — personalised concept slice from a question

```spl
SOLVE @style_guide  := style_instruction(@style)
GENERATE resolve_target(@question, concept_names())          -- NL question → concept node
ASSERT in_graph(@graph, @target) OTHERWISE GENERATE resolve_target(...)  -- re-resolve on miss
SOLVE @order := learning_path(@graph, @target, @learner_state, weight=1.0)  -- gap, productivity-ordered
WHILE @_i < len(@order) DO
    SOLVE @t_section_start := _now()                          -- timing: per-section wall clock
    GENERATE write_section(@concept, @concept, @style_guide)
    CALL verify_math(@section) → refine on "fail"
    LOGGING f"[timing] {@concept}: {_elapsed(@t_section_start):.1f}s"
END
GENERATE write_section(@target, @target, @style_guide)       -- close with the target itself
LOGGING f"[timing] workflow total: {_elapsed(@t_workflow_start):.1f}s"
```

Takes a free-text question (e.g. *"Why does diagonalization work?"*) and an
optional `@learner_state` (concepts already mastered), resolves it to a concept
node, computes exactly the gap the learner needs to fill — in teaching order —
and generates only that slice plus the target concept itself.

### `lean_payoffs.spl` — proof-grade post-pass (verifier ladder B-2)

Run *after* a build run; Lean stays off the default pipeline path. Per payoff
concept: formalize its canonical claim as a mathlib proposition
(`statement_ok` typecheck, capped repair loop) → LLM faithfulness judge,
where **UNFAITHFUL gates the badge** → citation-first proof
(`find_citation`: local `exact?`, then Loogle fallback, every candidate
kernel-checked) → on success, `cache_promote` the concept's cached section to
`machine_proved` with the statement stored for side-by-side audit. The build
run's CAS checks already earned `machine_verified`, so promoted entries carry
the first **two-axis badge sets**.

```bash
bash cookbook/tools/lean/setup_lean.sh --with-mathlib   # one-time, ~5 GB olean cache
spl-go run cookbook/71_linalg_concept_book/lean_payoffs.spl --kernel --llm claude_cli
spl-go cache list    # rank_nullity, diagonalization → machine_verified,machine_proved
```

Verified run (2026-06-11): `rank_nullity` and `diagonalization` promoted with
kernel-checked citations; `spectral_theorem` found no library citation and
correctly stayed `machine_verified` — failure withholds the badge, never
blocks delivery. Details: [`docs/DEV/spl3-lean.md`](../../docs/DEV/spl3-lean.md) §6.

## DODA: logical view → physical artifact

These `.spl` files are the **logical view**. The **physical artifact** is a
runnable Jupyter notebook produced by the deterministic `python/linalg` `splc`
transpiler — *no LLM call is needed to compile the structure*; only the
generated notebook's `GENERATE` cells call an LLM at run time:

```bash
spl-go splc compile cookbook/71_linalg_concept_book/build_concept_book.spl \
    --lang python/linalg

spl-go splc compile cookbook/71_linalg_concept_book/answer_on_demand.spl \
    --lang python/linalg
```

Each SPL construct maps deterministically to one or more notebook cells:

| SPL construct | Notebook cell |
|---|---|
| `WORKFLOW … DO … END` | markdown header + setup cell (imports `linalg_graph`, builds `graph`/`primitives`) |
| `CREATE FUNCTION` | prompt constant hoisted into a setup cell |
| `GENERATE fn(…) INTO @v` | code cell calling `_llm_call(prompt)` |
| `CALL proc(…) INTO @v` | code cell calling the Python function directly |
| `SOLVE @v TYPE := expr` | code cell: `v = expr` |
| `ASSERT expr [OTHERWISE …]` | code cell: `assert bool(expr)` + handler |
| `WHILE … DO … END` | code cell: `while` loop |
| `EVALUATE @v WHEN … THEN …` | code cell: `if`/`elif`/`else` |
| `COMMIT @v` | code cell: write output file + print |

"Changing domain = swap `linalg_graph.py`" — the same `.spl` logical structure
(graph-driven curriculum + style-guided generation + symbolic verification)
retargets to any domain that ships an equivalent concept-graph library.

## Run the compiled notebook

```bash
pip install networkx jupyter sympy

jupyter nbconvert --to notebook --execute \
    cookbook/71_linalg_concept_book/build_concept_book_python_linalg.ipynb
```

Set `LINALG_GRAPH_DIR=cookbook/71_linalg_concept_book` if the notebook can't
locate `linalg_graph.py` / `style_profiles.py` automatically, and `SPL_MODEL`
to choose the Ollama model used by the generated `_llm_call` helper.

## Sub-graph-scoped validation runs

`@target` naturally scopes the curriculum — `ancestors(@graph, @target)` pulls
in only the concepts `@target` transitively needs, so a target close to the
primitives produces a small curriculum that's cheap to run end-to-end. This is
the fastest way to validate a change (transpiler fix, prompt edit, cache
behavior, kernel config) *before* committing to the full ~13-section
`spectral_theorem` run (~10–25 min depending on cache state and model).

The compiled notebook reads every workflow `INPUT` scalar through
`_spl_config(KEY, default)` — env var → `~/.spl/config` → the `.spl`
`DEFAULT` — so `@target` (and `@style` / `@primitive_budget` /
`@payoff_weight`) can be overridden **without recompiling**:

```bash
cd cookbook/71_linalg_concept_book/targets/python_linalg

TARGET=linear_independence SPL_MODEL=gemma4:12b \
LINALG_GRAPH_DIR=/path/to/cookbook/71_linalg_concept_book \
jupyter nbconvert --to notebook --execute \
    --ExecutePreprocessor.kernel_name=spl123 \
    --ExecutePreprocessor.timeout=1800 \
    --output executed_smoke.ipynb \
    build_concept_book_python_linalg.ipynb
```

| `TARGET` | Ancestor concepts taught (sections) | Payoff capstone | Total LLM-authored blocks |
|---|---|---|---|
| `linear_independence` | 3 — `scalar_multiplication`, `vector_addition`, `linear_combination` | `linear_independence` | 4 |
| `diagonalization` | 8 — adds `linear_map`, `span`, `basis`, `eigenpair`, `linear_independence` | `diagonalization` | 9 |
| `spectral_theorem` (default) | 13 — full curriculum (both radicals, `⊕`/`⊙` and `⟨·,·⟩`, meet here) | `spectral_theorem` | 14 |

Other env vars useful for staged validation:

- `SPL_OLLAMA_URL` — point at a different Ollama daemon (default `http://localhost:11434`)
- `SPL_LLM_TIMEOUT` — per-`GENERATE` HTTP timeout in seconds (default `600`)
- `SPL_WHILE_MAX_ITER` — guard against runaway `WHILE` loops during development

**Recommended staging order**: smoke (`linear_independence`, 3 concepts) →
mid-size (`diagonalization`, 8 concepts, exercises a wider concept mix
including eigen-theory) → full (`spectral_theorem`, unset `TARGET` or set it
explicitly). At each stage, confirm: zero errors, correct cache HIT/MISS
counts (write-once — a HIT costs 0 LLM calls and is logged at `0.0s`), and a
well-formed `build_concept_book_output.md` (raw markdown/LaTeX — *not*
JSON-string-escaped; `COMMIT` routes `TEXT`-typed outputs straight to `.md`).

## Timing instrumentation

Both workflows log wall-clock elapsed time per section, per capstone, and for
the whole run — `[timing] {concept}: {elapsed:.1f}s` / `[timing] workflow
total: {elapsed:.1f}s` — exactly the data needed to profile and compare runs
once generation is spread across distributed workers (e.g. Momagrid).

The `.spl` source calls `_now()` / `_elapsed(start)` rather than `time.time()`
directly: the `SOLVE` python-template parser doesn't yet support dotted
attribute continuations like `module.attr(...)`, so these are bare-name
wrappers around `time.time()`, defined in the `python/linalg` setup cell
alongside `_verify_math` / `_shape_check` — the same "well-known helper"
mechanism, applied to timing instead of verification.

## Key learning points

1. **The graph is the curriculum.** `productivity_order` and `learning_path`
   make "what to teach next" a deterministic graph computation — reach-weighted
   topological sort — not an LLM judgment call.

2. **Style is orthogonal to truth.** One `style_guide` string threads through
   every `GENERATE` prompt; `verify_math`, `shape_check`, and `reducible` never
   see it and never change behavior because of it. Five very different reading
   experiences, one verified body of mathematics.

3. **Three independent refine loops, three different triggers** — `ASSERT
   new_primitives(@section) <= @primitive_budget OTHERWISE GENERATE
   refine_section(...)`, and `EVALUATE @check WHEN contains("fail") THEN
   GENERATE refine_section(...)` for both the math and shape checks. Each
   targets a different failure mode (pedagogical overload, factual error,
   dimensional inconsistency) with the same revise-in-place pattern.

4. **DODA in practice.** The `.spl` source compiles, with no LLM involvement,
   straight to a runnable notebook — proof that the *structure* of a
   neurosymbolic workflow is fully specified by the logical view, independent
   of which physical target executes it.
