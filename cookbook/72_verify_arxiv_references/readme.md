# Verify arXiv References

An automated citation-integrity checker. Given a `## References` block (as
found at the end of a paper draft), this recipe resolves every citation that
points at arXiv against the **actual arXiv record** — title, authors,
abstract — downloads the PDF, and writes a verifiable summary table. Citations
to books, journals, and conference proceedings (no arXiv id) are logged and
skipped, since there's no arXiv ground truth to check them against.

This automates exactly the kind of spot-check a careful reviewer does by hand:
Claude Opus 4.8 reviewed an early draft of our `neurosymbolic-spl` paper and
found that one citation (SymCode, arXiv:2510.25975) had been misattributed —
"Mukherjee, S. et al." instead of the real authors, Bagheri Nezhad et al. One
wrong author list is enough to make a reviewer distrust the whole reference
list. This recipe turns that single hand-check into a repeatable pipeline over
the entire bibliography.

---

## What it does

For each entry in `@in_refs`:

1. **Parse** — split the references block into individual entries (blank-line
   separated; hard-wrapped lines are rejoined).
2. **Extract** — pull an arXiv id out of `arXiv:XXXX.XXXXX` text or an
   `arxiv.org/abs|pdf/...` URL. No id found → log and skip (book/journal
   source — nothing to verify against).
3. **Look up ground truth** — query the public arXiv Atom API
   (`export.arxiv.org/api/query`) for the *actual* title, authors, and
   abstract. This is the source of truth the citation is checked against —
   **not** the citing paper's own description of the work.
4. **Download** — fetch the PDF into `@out_dir` (cached by filename,
   rate-limited to be polite to arXiv — same shape as
   `cookbook/47_arxiv_morning_brief`'s `download_arxiv_pdf`).
5. **Digest** — the LLM writes a **plain-prose digest of the verified
   abstract in fewer than 5 sentences** (`summary_desc`). The model only
   condenses; the facts (title, authors, abstract) come from arXiv, not
   from the model.
6. **Record** — append one row to `summary.csv`:
   `arxiv_id, title, summary_desc, authors`.
7. **Log skipped** — every skipped reference (no arXiv id, lookup/download
   failure, unexpected error) gets a row in `summary-skipped.csv`:
   `ref_index, arxiv_id, reason, ref_text`.

Everything is wrapped in a per-reference `DO ... EXCEPTION ... END` block
(mirroring recipe 47's per-paper loop): a lookup failure, a download error, or
an unexpected exception logs a warning and moves on to the next reference
rather than aborting the whole run.

All log messages are written to both console and a timestamped log file under
`~/.spl/logs/` via the `spl_log` TOOL_API — no SPL source is dumped into the
log, only the workflow's own `[LEVEL] message` lines.

---

## File structure

```
cookbook/72_verify_arxiv_references/
├── readme.md                      ← this file
├── verify_arxiv_references.spl    ← workflow + inline TOOL_API + LLM function
├── neurosymbolic-spl-refs.txt     ← real reference list (our own paper) for testing
└── output/                        ← default @out_dir: downloaded PDFs + summary.csv
```

The recipe is a **single self-contained `.spl` file** — `CREATE TOOL_API` and
`CREATE FUNCTION` blocks are defined inline rather than in a separate
`tools.spl` imported via `IMPORT`. (`IMPORT 'file.spl'` only registers
`WORKFLOW` definitions across files; the executor loads `TOOL_API`/`FUNCTION`
definitions solely from the top-level program's AST — see
`spl3/cli.py::run` and `spl3/_loader.py`.)

---

## Prerequisites

```bash
conda activate spl123
```

An LLM backend for the digest step — local Ollama is enough:

```bash
ollama serve                 # if not already running
ollama pull phi3             # or any small instruct model
```

(Steps 1–4 and 6 are deterministic Python tools — no LLM calls, no network
beyond arXiv. Only step 5, the digest, calls the model.)

---

## Running it

### Smoke test — single reference

```bash
spl-go run cookbook/72_verify_arxiv_references/verify_arxiv_references.spl \
    --adapter ollama --model phi3 \
    in_refs="## References

Smith, J. (2025). A test paper for citation verification. arXiv:2501.12948." \
    out_dir="cookbook/72_verify_arxiv_references/output"
```

Expected: 1 verified, `output/2501.12948.pdf` downloaded, and `summary.csv`
shows the *real* arXiv record — "DeepSeek-R1: Incentivizing Reasoning
Capability in LLMs via Reinforcement Learning" by DeepSeek-AI — which is **not**
the fake "Smith, J." citation fed in. That mismatch is exactly what this
recipe is built to surface.

### Full run — verify our paper's reference list

```bash
cd ~/projects/digital-duck/SPL.py
spl-go run cookbook/72_verify_arxiv_references/verify_arxiv_references.spl \
    --llm ollama:gemma3 \
    --param in_refs="cookbook/72_verify_arxiv_references/spl-dual-mode-refs.txt \
    --param out_dir="cookbook/72_verify_arxiv_references/output"
```

`neurosymbolic-spl-refs.txt` holds the real `## References` block from our
`neurosymbolic-spl` arXiv draft — 40 entries, 18 of which cite an arXiv id
(including the SymCode citation that triggered this recipe in the first
place). Expect the run to take a while: each verified entry does an API
lookup, a PDF download (rate-limited to ≥ 3 s between downloads), and an LLM
digest call. If arXiv responds `429 Too Many Requests` on a burst of lookups,
re-run — already-downloaded PDFs are cache-hits and skip the network fetch.

### Output

```
out_dir/
├── 2501.12948.pdf
├── 2510.25975.pdf
├── ...
├── summary.csv
└── summary-skipped.csv
```

```csv
arxiv_id,title,summary_desc,authors
2510.25975,"SymCode: <actual title>","<<5-sentence digest of the real abstract>","<actual author list>"
...
```

`summary-skipped.csv` records every reference that could not be verified:

```csv
ref_index,arxiv_id,reason,ref_text
3,,no_arxiv_id,"Russell, S. & Norvig, P. (2020). Artificial Intelligence: ..."
7,9999.99999,lookup_or_download_failed,"arXiv:9999.99999 — ..."
```

A timestamped log file is also written to `~/.spl/logs/verify_arxiv_references-YYYYMMDD-HHMMSS.log`.

Compare `summary.csv` row-by-row against the citations in
`neurosymbolic-spl-refs.txt` — any mismatch in title or authors is a
misattribution worth fixing before submission.

---

## Design notes

- **Ground truth lives outside the model.** Steps 1–4 and 6 are pure
  deterministic tools (regex parsing, an HTTP GET to a public API, a file
  write) — the kind of thing a type-checker or linter does. The LLM is invoked
  exactly once per verified reference, purely to *condense* prose that the
  tools already fetched and verified. This is the same fluency/correctness
  separation the parent paper argues for: the model never gets to invent a
  title or author list — it only rephrases one that arXiv already confirmed.
- **Skip ≠ fail.** A reference with no arXiv id (a book, a journal article)
  isn't an error — there's no arXiv ground truth to check it against, so it's
  logged at `DEBUG` and counted in `skipped`, not `verified`.
- **Cached, rate-limited downloads.** `download_pdf_to` checks for an
  existing non-empty file before hitting the network and sleeps between
  downloads — re-running the recipe after a partial failure (e.g. a `429`)
  only fetches what's missing.
