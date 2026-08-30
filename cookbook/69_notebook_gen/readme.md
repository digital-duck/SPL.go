# Recipe 69 — Jupyter Notebook Generator

**Pattern:** persistent IPython kernel (`--kernel`) + `CREATE FUNCTION` (LLM) +
`CREATE TOOL_API` notebook assembly (`tools.spl`)

## What this demonstrates

The IPython kernel as a **persistent, stateful execution substrate** woven through
an SPL workflow — and the LLM and kernel each staying in their own lane:

1. **Kernel state persists across `CALL` steps.** `import sympy as sp` and
   `A = sp.Matrix(...)` in step 1 are still live in steps 2 and 3 — no
   re-importing, no re-building the matrix.
2. **The kernel owns the math (exact); the LLM owns the narrative (language).**
   Eigendecomposition and the `A·v = λ·v` verification are computed and checked
   *symbolically* (SymPy equality, not floating-point), then handed to the LLM
   purely to explain in prose.
3. **Real `.ipynb` output.** The workflow writes an actual Jupyter notebook you
   can open and run immediately after `spl-go run` finishes.

## Architecture

```
CALL run_python('A = sp.Matrix(...)')         → kernel: build the matrix (state persists)
CALL run_python('eigendata = A.eigenvects()')  → kernel: deterministic eigendecomposition
CALL run_python('A * v == lam * v')            → kernel: symbolic verification A·v = λ·v
GENERATE write_intro()                         → LLM: 3-sentence section introduction
GENERATE explain_eigenvalues()                 → LLM: explain what the eigenpairs mean
CALL assemble_notebook()                       → tools.spl (TOOL_API): build & write the .ipynb
```

`assemble_notebook` (in `tools.spl`) keeps **all** text/JSON assembly in Python —
the workflow only ever passes simple values across the boundary, so no escaping of
LLM-generated prose is needed inside SPL f-strings.

## Files

```
cookbook/69_notebook_gen/
├── readme.md          ← this file
├── notebook_gen.spl   ← workflow + LLM prompt templates (IMPORT 'tools')
├── tools.spl          ← CREATE TOOL_API assemble_notebook(...) — builds the .ipynb
├── tools.py           ← equivalent @spl_tool implementation (loaded via --tools)
└── output/            ← default @output_path: the generated .ipynb
```

`tools.spl` and `tools.py` implement the **same** `assemble_notebook` tool two ways:
`tools.spl` registers it via `CREATE TOOL_API ... AS PYTHON $$ ... $$;` and is picked
up automatically through `IMPORT 'tools'` in `notebook_gen.spl`; `tools.py` registers
it via the `@spl_tool` decorator and is loaded with `--tools tools.py`. Either is
sufficient on its own — `notebook_gen.spl` uses the `IMPORT 'tools'` / `tools.spl`
path so the tool definition lives alongside the workflow as plain SPL source.

## Run

```bash
spl-go run cookbook/69_notebook_gen/notebook_gen.spl \
    --kernel --adapter claude_cli

# With a custom topic and matrix
spl-go run cookbook/69_notebook_gen/notebook_gen.spl \
    --kernel --adapter claude_cli \
    -p output_path=cookbook/69_notebook_gen/output/dot_prod.ipynb \
    -p topic="dot products and orthogonality" \
    -p matrix_a="[[1, 0], [0, -1]]"
```

Open the result:

```bash
jupyter notebook cookbook/69_notebook_gen/output/eigenvalues.ipynb
```

## Key learning points

1. **`--kernel` makes imports and variables durable across `CALL run_python`
   steps** — write `import sympy as sp; A = sp.Matrix(...)` once, reuse `A` and
   `eigendata` in every later step without re-deriving them.

2. **Symbolic equality, not floating-point comparison.** The verification step
   checks `bool(A * v == lam * v)` using SymPy's exact symbolic equality — the
   notebook's "Verified deterministically" claim is actually true, not "close enough."

3. **`CREATE TOOL_API` keeps prose assembly out of SPL f-strings.** Building a
   `.ipynb` means assembling JSON with embedded Markdown and code cells containing
   LLM-generated prose; doing that in Python (via `tools.spl`/`tools.py`) avoids
   any quoting/escaping headaches in the SPL layer.

4. **`IMPORT 'tools'` vs. `--tools tools.py`** — the same tool can be registered
   either as inline SPL (`CREATE TOOL_API ... AS PYTHON $$ ... $$;`, loaded via
   `IMPORT`) or as a standalone Python module (`@spl_tool`, loaded via `--tools`).
   This recipe ships both so you can compare the two registration paths directly.
