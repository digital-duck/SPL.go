# Recipe 73 — Intro Geometry Concept-Book Generator

**Pattern:** domain concept graph (`geometry_graph.py`) + style profiles
(`style_profiles.py`) + `SOLVE` / `ASSERT` (deterministic) + `GENERATE` /
`refine_section` (probabilistic) — compiled to a notebook via the
`python/intro_geometry` `splc` target (**DODA**: logical `.spl` view → physical
notebook artifact)

> **Status: spec-first.** The `.spl` workflows below validate cleanly
> (`spl-go validate`) and the domain library (`geometry_graph.py`,
> `style_profiles.py`) is smoke-tested standalone — but the
> `python/intro_geometry` `splc` transpiler **does not exist yet**
> (only `python/linalg` does, see `transpiler_linalg.py`). This recipe is
> therefore authored and validated as a **spec** ahead of its compile target,
> the same DODA move recipe 71 made for `python/linalg` before that
> transpiler landed. See "DODA: logical view → physical artifact" below and
> the corresponding entry in `README-todo.md`.

## What this demonstrates

A complete **neurosymbolic authoring pipeline** for an intro-geometry
concept-book, where a pre-built concept graph drives *what* gets taught and
in *what order* (deterministically), while the LLM only ever writes and revises
prose *within* the structure the graph guarantees is sound:

```
Deterministic (graph)   — verify the concept graph is acyclic and reducible to primitives
Deterministic (graph)   — compute the teaching order (topological + payoff-weighted reach)
Probabilistic  (LLM)    — write each section, in a chosen prose style
Deterministic (checks)  — verify numeric claims (SymPy geometry) — distances, angles, areas; refine on failure
Probabilistic  (LLM)    — write the capstone "Payoff" section tying the target to applications
```

This is **Layer 4** of the neurosymbolic-SPL roadmap: a domain library
(`geometry_graph.py`, Layer 2) plus two `WORKFLOW`s (Layer 4a/4b) that consume
it, intended to compile to a runnable artifact via a `python/intro_geometry`
`splc` transpiler (Layer 3 — **not yet built**, see Status above).

## The concept graph (`geometry_graph.py`)

A `networkx.DiGraph` of **29 nodes / 50 edges** encoding intro (plane)
geometry as a composition hierarchy, mirroring `linalg_graph.py`'s
"two radicals meet at the culmination" structure:

- **5 primitives** (tier 0, the irreducible "radicals"): `point`, `line`,
  `plane`, `distance`, `angle`. Two independent radicals — `distance` (the
  metric: how far) and `angle` (rotation: how much turn, `introduced_in:
  "trigonometry"`) — meet at `trigonometric_ratios`, the linalg recipe's
  `spectral_theorem` analogue.
- **17 concepts** (tiers 1–8), each declaring its `composed_of` prerequisites,
  e.g. `triangle = composed_of[line_segment, angle_measure, polygon]`,
  `trigonometric_ratios = composed_of[right_triangle-adjacent concepts,
  angle_measure, distance]`.
- **5 applications** (the payoff, pointing *out* of geometry):
  `navigation_surveying`, `architecture_construction`, `computer_graphics`,
  `astronomy`, `art_and_design`.

Edges `u → v` mean "u is a prerequisite of v". The graph algorithms
(`acyclic`, `reducible`, `ancestors`/`restrict`, `productivity_order`,
`gap`/`learning_path`, `applications_of`, `new_primitives`, …) are copied
**verbatim** from `linalg_graph.py` — they are domain-agnostic graph
operations, not linear-algebra-specific code. Only the graph's *content*
(nodes, edges, domain metadata) is geometry-specific.

A standalone smoke test confirms the structural narrative is not just
rhetorical: `reducible(graph, first_radical_primitives())` returns **`False`**
— `trigonometric_ratios` genuinely cannot be taught from `distance` alone, you
need `angle` too — while `reducible(graph, both_radical_primitives())` returns
`True`. Same proof shape as recipe 71's `⊕`/`⊙` vs. `⟨·,·⟩` for
`spectral_theorem`.

## Style profiles (`style_profiles.py`)

`@style` selects a **prose** profile — tone, depth, audience, length, structure
— without changing *what* is taught or *whether* it's correct.
`style_instruction(@style)` is resolved once via `SOLVE` and injected into
every `GENERATE` prompt as `{style_guide}`. The symbolic checks
(`verify_geometry`, `reducible`) are completely **style-agnostic** — only the
writing adapts.

This recipe ships recipe 71's five original profiles **unchanged**, plus two
new ones aimed at the audiences intro geometry actually serves day to day:

| Style | Audience | Structure |
|---|---|---|
| `textbook` | first-year university student | Definition → Worked example → Key theorem → Lab cell |
| `feynman` | curious person, no university math | Motivating story → Intuition → Minimal formalisation → "Now you try" |
| `flashcard` | student cramming the night before | Q: ... A: ... Example: ... (one fact per card) |
| `instructor` | instructor prepping a lecture | Concept summary → Common mistakes → Teaching tip → Exercise |
| `research` | grad student / researcher | Definition → Theorem → Proof → Remark (citation-ready) |
| `middle_school` *(new)* | 11–14, just starting geometry | Everyday example → Picture this → Simple rule → Try it yourself |
| `high_school` *(new)* | 14–18, algebra-1 background, SAT/ACT-bound | Real-world hook → Definition → Worked example → Why it's true → Practice problem |

**Are styles inclusive and additive?** Yes — and this recipe is the proof, not
just the claim. Adding `middle_school` and `high_school` required **zero**
changes to the original five profiles, to `style_instruction()`,
`available_styles()`, or to any verifier (`verify_math`/`verify_geometry`
never read `@style`). "Additive" because the *set* of choices only grows —
nothing is replaced or special-cased; "inclusive" because the two new profiles
extend the *audience range* the original five left uncovered (all five skew
university/adult — `textbook` assumes calculus, `research` assumes graduate
study, even `feynman`'s "curious adult" framing isn't a 12-year-old's frame of
reference). Geometry is exactly the domain where that gap matters: K-12
learners are its primary audience, not an edge case.

## Two workflows

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
    CALL verify_geometry(@section) → refine on "fail"        -- SymPy geometry-module recompute
    LOGGING f"[timing] {@concept}: {_elapsed(@t_section_start):.1f}s"
END
GENERATE write_payoff(@target, applications_of(@graph, @target), @style_guide)  -- capstone
LOGGING f"[timing] workflow total: {_elapsed(@t_workflow_start):.1f}s"
```

Builds a complete, dependency-ordered textbook from the primitives up to
`@target` (default: `trigonometric_ratios`), ending in a "Payoff" section that
ties the target concept to the real-world domains it unlocks.

### `answer_on_demand.spl` — personalised concept slice from a question

```spl
SOLVE @style_guide  := style_instruction(@style)
GENERATE resolve_target(@question, concept_names())          -- NL question → concept node
ASSERT in_graph(@graph, @target) OTHERWISE GENERATE resolve_target(...)  -- re-resolve on miss
SOLVE @missing := gap(@graph, @learner_state, @target)
SOLVE @path    := learning_path(@graph, @learner_state, @target)  -- gap, productivity-ordered
WHILE @_i < len(@path) DO
    GENERATE write_section(@concept, @learner_state, @style_guide)
    CALL verify_geometry(@section) → refine on "fail"
END
LOGGING f"Answer complete — {len(@path)} section(s), style: {@style}"
```

Takes a free-text question (e.g. *"Why does the Pythagorean theorem work?"*)
and an optional `@learner_state` (concepts already mastered), resolves it to a
concept node, computes exactly the gap the learner needs to fill — in teaching
order — and generates only that slice.

## What changed relative to recipe 71 — and what didn't

The two `.spl` files are adaptations, not rewrites. Everything that's
domain-agnostic infrastructure carries over **unchanged**:

| Carries over unchanged | Domain-specific — adapted |
|---|---|
| Content cache (`cache_get`/`cache_put`, `EVALUATE @section WHEN = "miss"`, write-once CAS) | Author persona: "expert linear algebra author" → "expert geometry author" / "geometry curriculum expert" |
| Timing instrumentation (`_now()`/`_elapsed()`, `[timing] …` logging) | Default `@target`: `spectral_theorem` → `trigonometric_ratios` (this graph's culmination) |
| `@primitive_budget` / `@payoff_weight` / `new_primitives` budget enforcement | **Verifier consolidation**: `verify_math` + `shape_check` (two checks) → single `verify_geometry` (one check) |
| `WORKFLOW` skeletons, `EVALUATE`/`ASSERT … OTHERWISE`/`GENERATE refine_section` refine loops | Prompts now ask for figures/diagrams to be *described in words* precisely enough to draw — geometry is visual in a way linear algebra's symbol manipulation isn't |

**Why one verifier instead of two:** recipe 71 needed both `verify_math`
(SymPy recompute of worked examples) *and* `shape_check` (matrix-dimension
consistency) because linear algebra has two independent failure modes —
"is the arithmetic right" and "do these matrices even compose." Geometry has
no matrix-shape analogue — there's nothing to dimension-check — so
`verify_geometry` (a SymPy `geometry`-module recompute of every numeric claim:
distances, angle measures, areas, the Pythagorean relationship, slopes,
similarity ratios) is the **single** verification gate, geometry's direct
counterpart to `verify_math`.

## DODA: logical view → physical artifact

These `.spl` files are the **logical view**. Per DODA, that's a complete and
independently meaningful artifact — `spl-go validate` confirms the workflow
structure, function signatures, and control flow are sound — *before* any
compile target exists for it:

```bash
spl-go validate cookbook/73_intro_geometry/build_concept_book.spl
spl-go validate cookbook/73_intro_geometry/answer_on_demand.spl
```

Both currently report `OK` with only the same class of flow-analysis warnings
(`@var used before assignment`, `WHILE … no max_iterations`, `CALL target …
not found in CREATE FUNCTION declarations`) that `spl-go validate` *also* raises
against the upstream, working `cookbook/71_linalg_concept_book/*.spl` files —
i.e. these are pre-existing validator quirks (its flow analyzer doesn't yet
trace `EVALUATE`/`WHILE` bindings or recognize domain-target setup-cell
helpers), not defects introduced here.

The **physical artifact** — once `python/intro_geometry` exists — would be a
runnable Jupyter notebook produced the same deterministic way recipe 71's is:

```bash
conda activate spl123
pip install ipykernel
python -m ipykernel install --user --name spl123 --display-name "Python (spl123)"

spl-go splc compile cookbook/73_intro_geometry/build_concept_book.spl \
    --lang python/intro_geometry      # ← target doesn't exist yet

spl-go splc compile cookbook/73_intro_geometry/answer_on_demand.spl \
    --lang python/intro_geometry      # ← target doesn't exist yet


# run notebook
jupyter notebook cookbook/73_intro_geometry/targets/python_intro_geometry/answer_on_demand_python_intro_geometry.ipynb
# select spl123 kernel


```

Each SPL construct would map deterministically to one or more notebook cells,
exactly as it does for `python/linalg` (see recipe 71's readme for the full
construct → cell table) — `WORKFLOW` → setup cell importing `geometry_graph`
and building `graph`/`primitives`, `GENERATE` → `_llm_call(prompt)` cell,
`CALL verify_geometry(...)` → direct Python call, etc. **"Changing domain =
swap `linalg_graph.py` for `geometry_graph.py`"** is the thesis recipe 71's
readme states; this recipe is the first real test of that thesis once a
transpiler exists to compile it.

### The transpiler gap (tracked in `README-todo.md`)

`transpiler_linalg.py` currently *hardcodes* its domain: it imports
`linalg_graph` by name, calls `lg.both_radical_primitives()`, and bakes
`_verify_math`/`_shape_check`/`_now`/`_elapsed` into its `_SETUP_TEMPLATE`.
Standing up `python/intro_geometry` therefore means one of:

1. **Write `transpiler_intro_geometry.py`** — a sibling module mirroring
   `transpiler_linalg.py`'s structure, swapping in `geometry_graph` /
   `verify_geometry` / the geometry primitives accessor. Fast to ship,
   but repeats recipe 71's "is duplication OK?" question one level up
   the stack — now at the *transpiler* layer, not just the domain-library
   layer.
2. **Generalize `transpiler_linalg.py`** into a parameterized
   `python/<domain>` target — pass in the graph-module name, the
   primitives accessor, and the verifier function name(s) (one for
   linalg's two-check shape, one for geometry's single-check shape) as
   target configuration rather than hardcoded imports. More work up
   front; pays for itself the moment a *third* domain (e.g. `73`'s
   sibling recipes on trigonometry or classical mechanics) needs a target.

This recipe doesn't resolve that question — it just makes the question
concrete and unblocks answering it with a real second domain to generalize
against, instead of a hypothetical one.

## Run the compiled notebook

Not runnable yet — see "The transpiler gap" above. Once
`python/intro_geometry` exists, this section will mirror recipe 71's://
`pip install networkx jupyter sympy`, then `jupyter nbconvert --to notebook
--execute cookbook/73_intro_geometry/build_concept_book_python_intro_geometry.ipynb`,
with `GEOMETRY_GRAPH_DIR` / `SPL_MODEL` env-var overrides for locating
`geometry_graph.py` / `style_profiles.py` and choosing the LLM.

## Timing instrumentation

Both workflows log wall-clock elapsed time per section, per capstone, and for
the whole run — `[timing] {concept}: {elapsed:.1f}s` / `[timing] workflow
total: {elapsed:.1f}s` — identical in spirit and mechanism to recipe 71's:
`_now()`/`_elapsed(start)` bare-name wrappers around `time.time()` (a
workaround for the `SOLVE` python-template parser's lack of dotted-attribute
continuation support), defined in the (future) `python/intro_geometry` setup
cell's "well-known helper" template alongside `_verify_geometry`.

## Key learning points

1. **The graph is the curriculum — and the algorithms travel.**
   `geometry_graph.py`'s graph-algorithm functions (`productivity_order`,
   `learning_path`, `gap`, `reducible`, …) are byte-for-byte copies of
   `linalg_graph.py`'s. "What to teach next" is a domain-agnostic graph
   computation; only the *content* of the graph is domain-specific.

2. **Style is additive by construction — proven, not asserted.** This
   recipe adds two new audiences (`middle_school`, `high_school`) to the
   five it inherits, with zero edits to the inherited five, to
   `style_instruction()`, or to any verifier. The libraries don't merely
   *permit* extension — extending them required touching nothing that
   already worked.

3. **One domain, one verifier shape — and that's fine.** Recipe 71 needed
   two checks (`verify_math` + `shape_check`) because linear algebra has two
   independent failure surfaces. Geometry has one (`verify_geometry`). The
   `.spl` *workflow* structure accommodates both shapes without forcing a
   lowest-common-denominator verifier interface — each domain declares
   exactly the checks its content needs.

4. **DODA, stress-tested.** A `.spl` source can be authored, reviewed, and
   `spl-go validate`-clean *before* its compile target exists — proof that the
   logical view is a complete, independently meaningful artifact, and that
   "spec first, transpiler second" is a normal, supported way to grow the
   cookbook past its current ~71 recipes toward 100 and beyond.
