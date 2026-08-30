# Recipe 95 — Music Theory: Four-Part Chorale Voice-Leading Verification

**Category:** reasoning · **Tier:** 2 · **Requires:** stdlib `json` only — no new pip dependency

## What this demonstrates

Suggested by Qwen (2026-07-25, see `docs/publication/TMLR/SPL-for-Agentic-Workflow/review-feedback/LLM/qwen-3-8-max-full-20260725.md`) as the "unexpected domain" — the demonstration that answers "does this generalize beyond math?" with an audible, viscerally understandable *no, it doesn't stay in STEM*. An LLM can generate a four-part chorale that *looks* stylistically plausible (probabilistic — pattern-matching on training data), but classical voice-leading rules (no parallel fifths/octaves, no voice crossing, bounded adjacent-voice spacing) are hard constraints that LLMs routinely violate and cannot reliably self-check in prose.

**Score representation:** a JSON list of `{"S","A","T","B"}` MIDI pitch numbers, one entry per beat — chosen deliberately over MusicXML + `music21` to keep this recipe stdlib-only, consistent with every other cookbook recipe (`music21` is not installed in this environment; wiring it in as the deterministic checker's backend is a reasonable future upgrade behind the same `CREATE TOOL_API` boundary, without touching the `.spl` workflow).

| Stage | Mode | Tool | Why |
|-------|------|------|-----|
| Harmonize melody | **Probabilistic** | LLM (`harmonize`) | Four-part setting of a given soprano line |
| Check voice-leading | **Deterministic** | `check_voice_leading()` | Parallel 5ths/8ves, voice crossing, adjacent-voice spacing, large leaps — pure interval arithmetic |
| Repair loop | **Probabilistic + Deterministic** | `WHILE` + `fix_violations` | LLM revises A/T/B only (soprano must stay fixed); kernel re-checks, up to `max_tries` |
| Gate on clean | **Deterministic** | `ASSERT` | No violations remain |
| Melody round-trip | **Deterministic** | `classify_roundtrip()` | Confirms the LLM did not "fix" a violation by silently altering the *given* soprano melody itself |
| Interpret result | **Probabilistic** | LLM (`annotate`) | Plain-English harmonic analysis of the verified score |

**Key property:** the round-trip check here is domain-specific in a way none of the other recipes' checks are — it isn't re-verifying the *answer*, it's re-verifying that the LLM didn't cheat on the *constraint* (the input melody) while satisfying the *other* constraint (voice-leading). This is a genuinely different failure mode than a wrong numeric answer: a "clean" score that silently changed the given melody would pass `ASSERT` but fail `classify_roundtrip`.

## `enable_solver=false` vs `enable_solver=true`

- **`enable_solver=false`** (ARM B, unaided baseline): the LLM harmonizes and self-reports on voice-leading correctness in prose, with no deterministic checker — exactly where LLMs confidently describe a parallel fifth they did not actually avoid.
- **`enable_solver=true`** (ARM A, default): LLM harmonizes; `check_voice_leading()` checks parallel 5ths/8ves, crossing, spacing, and leaps; repair loop (LLM revises A/T/B only, soprano frozen) up to `max_tries`; `ASSERT` gates on zero violations; `classify_roundtrip()` confirms the soprano voice still matches the original input note-for-note; LLM narrates the verified score.

## Run

```bash
# Default melody: C5 D5 E5 F5 G5 F5 E5 C5 (MIDI: 72,74,76,77,79,77,76,72)
spl-go run cookbook/95_music_theory/music_theory.spl --llm claude_cli

# Custom melody
spl-go run cookbook/95_music_theory/music_theory.spl --llm claude_cli \
    --param melody="60,62,64,65,67,65,64,60"

# Unaided baseline arm
spl-go run cookbook/95_music_theory/music_theory.spl --llm claude_cli --param enable_solver=false
```

## Verified (2026-07-26)

- `check_voice_leading()` unit-tested directly (independent of any LLM): correctly flags parallel fifths + octaves on a textbook root-position-triad-moved-up-a-step case, correctly passes a contrary-motion clean case, correctly flags voice crossing.
- Full pipeline exercised end-to-end with `ollama:rnj-1`: the model's first harmonization attempt moved all four voices in exact parallel motion (blocked triads) — a real, textbook-classic mistake — and `check_voice_leading()` correctly caught it: `parallel_octaves` (S/B) and `parallel_fifths` (A/B) at beats 1, 2, and 7. This is a clean positive validation of the checker against a genuine, unprompted LLM error, not a synthetic test case.
- The repair loop (`fix_violations` + re-check) did not converge within 2 attempts on `rnj-1` — the `EXCEPTION WHEN ToolFailed` path fired correctly, wrote a graceful failure report, and returned `status=error` rather than a false "clean" result. Plumbing (`WHILE`/`EVALUATE`/`GENERATE`/`ASSERT`/`EXCEPTION`) is fully validated; getting the *repair* to reliably converge needs a stronger model. `claude_cli` end-to-end verification is pending — hit Anthropic's session rate limit during testing; retry recommended when available.

## Notes on model capacity

Unlike the math/planning/SQL recipes, this one asks the model to re-emit an entire `beat_count`-length JSON structure (4 voices × N beats) on every repair turn, not just a small delta — a meaningfully larger structured-output burden, and genuinely revising A/T/B to remove a specific flagged parallel-fifths pair requires more compositional understanding than translating a math problem into `expr|op`. If a model's repair loop doesn't converge, consider: (a) raising `--llm-max-output-tokens`, (b) shortening the default melody below 8 beats, or (c) a stronger model (`claude_cli` or `qwen3` are the strongest performers in the paper's own r=6 results). This is itself a small, incidental data point for the paper's F2 finding (format-compliance capability varies by model) — worth noting if this recipe is ever benchmarked quantitatively, not just used as a qualitative demo.

## Execution flow

```
GENERATE harmonize(@reference, @melody)                 -- LLM harmonizes (S frozen = given melody)
    │
CALL check_voice_leading(@score)                         -- parallel 5ths/8ves, crossing, spacing, leaps
    │
WHILE NOT clean AND @tries < @max_tries                  -- repair loop, A/T/B only
    │      fix_violations() -> check_voice_leading()
    ↓
ASSERT clean                                              -- hard gate
    │
CALL classify_roundtrip(@melody, @score)                  -- soprano == original input, note-for-note
    │
GENERATE annotate(@score, @solution)                      -- LLM narrates the verified score
    │
CALL format_report(...)                                   -- Markdown report
```


