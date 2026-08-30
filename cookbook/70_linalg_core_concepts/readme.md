# Recipe 70 — Linear Algebra Core Concepts (Content-Cache Experiment)

**Pattern:** `CREATE TOOL_API` (deterministic context/term lookup + coverage
check) + `CREATE FUNCTION` (LLM prose + refine) + the **Layer 2 content
cache** (`cache_get` / `cache_put`) — a content-addressed, write-once store
that lets two independently-run workflows converge on one shared, verified
body of prose

## What this demonstrates

A **trimmed-down** concept-book generator — just enough machinery to test
the basic framework before tackling [recipe 71](../71_linalg_concept_book),
which drives the same kind of pipeline off a 37-node concept graph. Where
recipe 71 asks "does the *whole curriculum* compile to a notebook?", recipe 70
asks a narrower question first: **"does verified content survive being asked
for twice, by two different workflows, in two different runs — without a
second LLM call?"**

Two scripts generate (overlapping slices of) Axler's *Linear Algebra Done
Right* (4e), Chapter 1 "Vector Spaces":

| Script | Generates | Cache role |
|---|---|---|
| `chapter-1-vector-spaces-1A-real-complex.spl` | section **1A** (`Rn and Cn`) only | writes the *first* cache entry |
| `chapter-1-vector-spaces-1C-subspaces.spl` | the *whole chapter*: **1A, 1B, 1C** | reuses 1A from cache; writes 1B, 1C fresh |

Run them in order against the **same** `.spl/content_cache.db`:

```
1. 1A script alone   → cache MISS  → generate, verify, cache_put('1A_Rn_and_Cn', ...)
2. 1C script (whole) → cache HIT   on 1A_Rn_and_Cn  (0 LLM calls, 0 refine passes)
                     → cache MISS  on 1B, 1C        (generated, verified, cached fresh)
3. 1C script again   → cache HIT   on all three     (0 LLM calls at all)
```

This is the **same cache key resolving the same way regardless of which
workflow asks** — the point of a content-addressed cache: verified prose is
write-once, immutable, and de-duplicated by *(concept, params, rubric_version,
dep_hashes)*, never by "which script generated it" or "in what order".

## Architecture

```sql
CALL section_context(@concept)  → deterministic: concept id → book-context string fed to the LLM
CALL section_terms(@concept)    → deterministic: concept id → required-terms checklist
CALL cache_get(@concept)        → deterministic: CAS lookup; returns content or sentinel "miss"

EVALUATE @section
  WHEN = 'miss' THEN                              -- deterministic string equality (NOT semantic!)
    GENERATE write_section(@concept, @book_context)  → LLM: 250-350 word section, LaTeX, worked example
    CALL check_terms(@section, @required_terms)      → deterministic: do all required terms appear?
    EVALUATE @check
      WHEN contains('fail') THEN GENERATE refine_section(...)  -- LLM: targeted revision
      ELSE  -- coverage passed
    END
    CALL cache_put(@concept, @section)              → deterministic: store, return CAS key
  ELSE
    -- cache HIT: reuse verbatim, 0 LLM calls
END
```

## A deterministic check, not a judgment call

`cache_get` returns the literal string `"miss"` on a cache miss. The dispatch
on that sentinel is written as:

```sql
EVALUATE @section
  WHEN = 'miss' THEN ...
```

**Not** `WHEN 'miss' THEN ...`. A bare string literal parses as a
`SemanticCondition` — it asks the LLM "does this text look like it matches
'miss'?", which is exactly the kind of category error this cookbook series
warns against: turning a deterministic equality check on a known sentinel
into a probabilistic judgment call. `WHEN = 'miss'` parses as a
`ComparisonCondition` and falls back to plain string equality — fast, free,
and correct every time. The same distinction shows up in `WHEN
contains('fail')`, which is a deterministic substring check on the
`check_terms` output, not an LLM asked to opine on whether something "seems
like a failure".

## Why two near-identical scripts?

Both scripts ship an **identical copy** of `section_context`, `section_terms`,
and `check_terms` — a miniature stand-in for recipe 71's `linalg_graph.py`.
This is intentional, not duplication-for-its-own-sake: each script must be
runnable standalone, and — critically — the *same* `@concept` id must resolve
to the *same* book-context string in both, so that `content_key(concept,
params, rubric_version, dep_hashes)` produces the *same* cache key no matter
which script computes it. Diverge the lookup tables and the cache "hit" would
silently serve content generated from different source material — the
content-addressing would be correct but the experiment would be meaningless.

## Run

Use a scratch working directory so the `.spl/content_cache.db` /
`.spl/content_meta.db` files don't pollute the repo (they're cwd-relative and
git-ignored):

```bash
mkdir -p /tmp/spl_cache_test && cd /tmp/spl_cache_test

# 1. Generates + caches section 1A (cache MISS)
spl-go run /path/to/cookbook/70_linalg_core_concepts/chapter-1-vector-spaces-1A-real-complex.spl \
    --adapter ollama --model gemma3

# 2. Reuses 1A from cache (HIT, 0 LLM calls); generates + caches 1B and 1C (MISS)
spl-go run /path/to/cookbook/70_linalg_core_concepts/chapter-1-vector-spaces-1C-subspaces.spl \
    --adapter ollama --model gemma3

# 3. Run again — now ALL THREE are cache hits
spl-go run /path/to/cookbook/70_linalg_core_concepts/chapter-1-vector-spaces-1C-subspaces.spl \
    --adapter ollama --model gemma3
```

Watch the log lines to see the cache state machine in action:

```
[INFO] cache MISS for 1A_Rn_and_Cn — generating with the LLM
[INFO] Coverage check passed
[INFO] Stored verified section — cache key spl3:content:ec3e5724...

[INFO] cache HIT for 1A_Rn_and_Cn — reusing verified section (0 LLM calls, 0 refine passes)
[INFO] cache MISS for 1B_definition_of_vector_space — generating with the LLM
[INFO] cache MISS for 1C_subspaces — generating with the LLM
```

To inspect or reset the cache between experiments:

```bash
sqlite3 .spl/content_cache.db "SELECT key, length(content) FROM content;"
rm -rf .spl    # full reset — next run starts from all-MISS again
```

## Timing instrumentation

Both scripts log wall-clock elapsed time per section and for the whole
workflow — `[timing] {concept}: {ms} ms (generated|cached)` and `[timing]
workflow total: {ms} ms` — via two small deterministic tools, `now_ms()` and
`elapsed_ms(start_ms)` (`CREATE TOOL_API ... AS PYTHON`, called with `CALL
... INTO`). Run with `--adapter echo` you'll see cache hits cost a few ms and
misses cost ~50ms (one templated round trip); with a real LLM the gap between
"cached" and "generated" widens by orders of magnitude — exactly the signal
worth capturing before scaling generation across distributed workers (e.g.
Momagrid), where knowing which sections are cache hits vs. fresh LLM calls
determines how work should be scheduled and split.

[Recipe 71](../71_linalg_concept_book) instruments the same way but through
`_now()` / `_elapsed()` kernel helpers (via `SOLVE`, since its workflows are
compiled to notebooks rather than run directly) — same signal, different
plumbing for a different execution substrate.

## Key learning points

1. **The cache is content-addressed, not workflow-addressed.** Two
   independently-authored `.spl` files, run in sequence, converge on one
   shared verified section the moment their `(concept, params,
   rubric_version, dep_hashes)` tuples match — no coordination between them
   required, just agreement on the *identity* of what they're asking for.

2. **`= 'miss'` vs `'miss'` is the whole ballgame.** Comparing a known
   sentinel string with `WHEN =` keeps a structural decision deterministic;
   writing it as a bare string literal silently converts it into an LLM call.
   The cost isn't just latency — it's that "was this a cache hit?" becomes a
   probabilistic question with a non-zero chance of being wrong.

3. **Verification gates what gets cached, not what gets generated.**
   `check_terms` → `refine_section` runs *before* `cache_put` — only content
   that has passed the deterministic coverage check is written to the
   immutable store. A cache hit is therefore a guarantee of "previously
   verified", not just "previously generated".

4. **This is the floor, not the ceiling.** Recipe 70 hand-rolls a two-entry
   lookup table where recipe 71 walks a 37-node graph; it checks for required
   terms where recipe 71 recomputes worked examples with SymPy. Once the
   cache state machine is proven here — MISS → generate → verify → store,
   HIT → reuse, convergence across independent callers — scaling it up to
   the full concept graph in recipe 71 is "more of the same", not "something
   new that might not work".
