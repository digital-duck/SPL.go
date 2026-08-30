# arXiv Morning Brief

A daily AI workflow that downloads arXiv PDFs, semantically chunks each paper,
summarizes every section with an LLM, reduces to a per-paper abstract, then
formats everything into a clean Markdown morning brief.

Built with **SPL-by-Spec** — see `SPEC.md` for the full design and `specs/flowchart.pdf`
for the data flow diagram.

---

## File Structure

```
cookbook/arxiv_morning_brief/
├── readme.md                    ← this file
├── SPEC.md                      ← full design spec (SPL-by-Spec)
├── arxiv_morning_brief.spl      ← top-level orchestrator workflow
├── summarize_paper.spl          ← per-paper sub-workflow
├── functions.spl                ← LLM function definitions (chunk_summarizer etc.)
├── tools.py                     ← Python tool implementations
├── specs/
│   ├── flowchart.tex            ← TikZ source
│   ├── flowchart.pdf            ← rendered data flow diagram
│   └── readme-tikz.md           ← how to convert PDF → SVG / PNG
└── tests/
    ├── conftest.py              ← shared fixtures
    ├── fixtures/
    │   └── expected_chunks.json ← expected chunk structure
    ├── test_tools.py            ← Level 1: unit tests (16 tests)
    └── test_workflow.py         ← Level 2: workflow dry-run (4 tests)
```

---

## Prerequisites

### Conda environment

```bash
conda activate spl2          # the environment where spl-llm, dd-* are installed
```

Verify the key packages are present:

```bash
python -c "import spl; import spl; import dd_extract; import dd_cache; print('OK')"
```

### Environment variables

Set at least one LLM backend before running for real:

```bash
# Local Ollama (no key needed)
# just make sure `ollama serve` is running


# Anthropic (Claude)
export ANTHROPIC_API_KEY=sk-ant-...

# OpenRouter (multi-model)
export OPENROUTER_API_KEY=sk-or-...
```

---

## Running the Tests

### Level 1 — Unit tests (tools.py, no LLM, no network)

```bash
conda activate spl2
cd /home/papagame/projects/digital-duck/SPL30/cookbook/arxiv_morning_brief
python -m pytest tests/test_tools.py -v
```

Expected: **16 passed**. Tests cover:
- `download_arxiv_pdf`: cache hit, rate limiting (≥ 3s), HTTP error → ToolError
- `semantic_chunk_plan`: header-based chunking, paragraph fallback, missing PDF, empty extraction
- `list_count` / `get_item` / `list_append`: correctness and edge cases
- `build_brief_date_header`: explicit date, empty → today, whitespace → today

### Level 2 — Workflow dry-run (SPL orchestration, mock LLM)

```bash
python -m pytest tests/test_workflow.py -v
```

Expected: **4 passed**. Tests cover:
- `summarize_paper` sub-workflow produces a non-empty summary
- `arxiv_morning_brief` orchestrator produces a Markdown brief
- ToolError from download → paper skipped, brief still produced
- Empty URL list → brief still produced (no papers section)

### All tests together

```bash
python -m pytest tests/ -v
```

Expected: **20 passed in ~7s**

---

## Running the Agent for Real

### Minimal run (stdout output)

```bash
conda activate spl2
cd /home/papagame/projects/digital-duck/SPL30/cookbook/arxiv_morning_brief

spl-go run arxiv_morning_brief.spl \
    --param urls='["https://arxiv.org/pdf/2501.12948","https://arxiv.org/pdf/2501.12345"]' \
    --param date='2026-03-31'
```

Output is printed to stdout as Markdown. Pipe to a file:

```bash
spl-go run arxiv_morning_brief.spl \
    --param urls='["https://arxiv.org/pdf/2501.12948"]' \
    > ~/morning-brief-$(date +%F).md
```

### With explicit model

```bash
spl-go run arxiv_morning_brief.spl \
    --model claude-sonnet-4-6 \
    --param urls='["https://arxiv.org/pdf/2501.12948"]'
```

### Momagrid dispatch (LAN GPU grid)

```bash
spl-go run arxiv_morning_brief.spl \
    --adapter momagrid --hub http://momagrid.org:9000 \
    --param urls='arxiv-papers.txt' \
    --param date='2026-03-31'

spl-go run arxiv_morning_brief.spl \
    --adapter momagrid --hub http://momagrid.org:9000 \
    --param urls="https://arxiv.org/abs/2602.15860 https://arxiv.org/abs/2601.09732, https://arxiv.org/abs/2602.21257" \
    --param date='2026-03-31'

```

Sub-workflows are dispatched sequentially by default.
`CALL PARALLEL` fan-out is flagged for SPL 3.1 — see SPEC.md § 9 Open Questions.

---

## How the Workflow Runs

```
arxiv_morning_brief.spl
  │
  ├── CALL build_brief_date_header()      ← tools.py  (date header)
  ├── WHILE each URL
  │     └── CALL summarize_paper()        ← sub-workflow via WorkflowComposer
  │           ├── CALL download_arxiv_pdf()   ← tools.py (dd-cache, 3s rate limit)
  │           ├── CALL semantic_chunk_plan()  ← tools.py (dd-extract, structural)
  │           ├── WHILE each chunk
  │           │     └── GENERATE chunk_summarizer()   ← LLM (functions.spl)
  │           └── GENERATE paper_reducer()            ← LLM (functions.spl)
  └── GENERATE brief_writer()             ← LLM (functions.spl)
```

PDF cache lives at `~/.cache/dd_arxiv_morning_brief/` (TTL 24h).
Subsequent runs for the same URLs on the same day skip the download.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| `ModuleNotFoundError: spl` | Wrong conda env | `conda activate spl2` |
| `ToolError: HTTP 429` | arXiv rate limit hit | Increase `_RATE_LIMIT_SECS` in tools.py (default 3s) |
| `ToolError: No text extracted` | Scanned/image PDF | Use `PDFExtractor(engine="docling")` in tools.py |
| `MaxIterationsReached` | Very long paper (>25 chunks) | Add `--param max_tokens=256` to reduce chunk summaries |
| Pyright import errors in tests | Packages in conda, not Pyright venv | Ignore — all 20 tests pass at runtime |

---

## Level 3 — Integration Test (real network, real LLM)

Gated on `ARXIV_INTEGRATION=1` to avoid accidental API spend:

```bash
ARXIV_INTEGRATION=1 python -c "
import asyncio, json, sys
sys.path.insert(0, '.')
from spl._loader import load_workflows_from_file
from spl.registry import LocalRegistry
from spl.composer import WorkflowComposer
from spl.executor import SPL3Executor
from spl.adapters.anthropic import AnthropicAdapter
import tools  # registers @spl_tool functions

adapter  = AnthropicAdapter()
executor = SPL3Executor(adapter=adapter)
registry = LocalRegistry()
for defn in load_workflows_from_file('arxiv_morning_brief.spl'):
    registry.register(defn)
executor.composer = WorkflowComposer(registry, executor)

defn   = registry.get('arxiv_morning_brief')
result = asyncio.run(executor.execute_workflow(defn.ast_node, params={
    'urls':  json.dumps(['https://arxiv.org/pdf/2501.12948']),
    'date':  '2026-03-31',
}))
print(result.committed_value)
assert len(result.committed_value) > 200, 'Brief too short'
print('Integration test passed.')
"
```
