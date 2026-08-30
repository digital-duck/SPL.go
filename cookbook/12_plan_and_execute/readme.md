# Recipe 12: Plan and Execute

A planner agent decomposes a complex task into steps; an executor runs each step sequentially with validation. Failed steps trigger automatic re-planning. When `output_dir` is set, generated code files are written to disk.

## Usage philosophy: generate → review → fix → iterate

This recipe produces a **first draft**, not production-ready code. That is intentional.

The value of the plan-and-execute pattern is not to ship code directly — it is to:

- **Eliminate the blank-page problem** — a coherent file structure, API surface, and data models appear in seconds
- **Make the design explicit** — each step's design notes are visible in the output, so you can see *why* each file exists
- **Give you something concrete to fix** — it is far faster to correct a generated skeleton than to write it from scratch

**The intended workflow:**

```
spl-go run → review generated files → fix inconsistencies → test → iterate
```

Never assume generated code runs without review. Common things to check:

- All files use the same framework (no drift between steps)
- Imports between files are correct
- `requirements.txt` lists actual pip packages (not stdlib modules)
- No module-level code that requires app context (e.g., `db.create_all()`)
- Entry point wires together all the other modules

The stronger the model, the fewer corrections needed — but the review step is always required.

---

## Pattern

```
plan(task) → @plan
  └─► count_steps(@plan, max_steps) → N        ← capped to prevent runaway loops
        └─► for each step:
              extract_step → execute_step (brief design notes only)
                ├─ failed  → replan → restart
                └─ ok      → accumulate lightweight results
                      └─► outline_files(task, results) → @file_outline   ← filenames only
                              └─► for each file:
                                    extract_file → generate_file          ← one file per LLM call
                                      └─► write_code_files(@file_code)   ← written immediately
                                              └─► summarize → @final_report
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `task` | TEXT | `Build a REST API for a todo app` | The complex task to decompose and execute |
| `output_dir` | TEXT | `''` | Directory to write generated code files (empty = skip) |
| `max_steps` | INTEGER | `5` | Maximum number of plan steps (guards against runaway loops) |

## Usage

```bash
# Basic planning only (no code output)
spl-go run cookbook/12_plan_and_execute/plan_execute.spl --adapter ollama \
    task="Build a REST API for a todo app"

# Generate and write code to disk (one file per LLM call — reliable for any model)
spl-go run cookbook/12_plan_and_execute/plan_execute.spl \
    --adapter ollama --model "qwen2.5-coder" \
    --tools cookbook/12_plan_and_execute/tools.py \
    task="Build a REST API for a todo app" \
    output_dir="cookbook/12_plan_and_execute/output" \
    max_steps=4 \
    2>&1 | tee cookbook/out/12_plan_and_execute-ollama-qwen2.5-$(date +%Y%m%d_%H%M%S).md

# code quality is bad, use Claude Code

spl-go run cookbook/12_plan_and_execute/plan_execute.spl \
    --adapter claude_cli --model claude-sonnet-4-6 \
    --tools cookbook/12_plan_and_execute/tools.py \
    task="Build a REST API for a todo app" \
    output_dir="cookbook/12_plan_and_execute/output" \
    max_steps=4 \
    2>&1 | tee cookbook/out/12_plan_and_execute-claude-sonnet-$(date +%Y%m%d_%H%M%S).md
# see log
# /home/gongai/projects/digital-duck/SPL20/cookbook/out/12_plan_and_execute-claude-sonnet4.6-20260321_025923.md

# see /home/gongai/projects/digital-duck/SPL20/cookbook/12_plan_and_execute/output-claude-sonnet4-6/README.md

spl-go run cookbook/12_plan_and_execute/plan_execute.spl --adapter ollama \
    task="Set up a CI/CD pipeline for a Python project"

spl-go run cookbook/12_plan_and_execute/plan_execute.spl --adapter ollama \
    task="Migrate a MySQL database to PostgreSQL"
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | All steps executed successfully |
| `partial` | Max iterations hit mid-execution |
| `budget_limit` | Token budget exceeded |

## Lessons learned: designing WHILE loops in SPL

This recipe went through three iterations, each exposing a different anti-pattern. The progression is worth studying because the same mistakes appear in almost every agentic workflow.

---

### Anti-pattern 1: heavy generation inside the loop

**What went wrong (v0):**

The original `execute_step` asked the LLM to produce full production code on every loop iteration:

```spl
WHILE @step_index < @step_count DO
    GENERATE execute_step(@current_step, @results) INTO @step_result  -- ← full code output
    @results := @results + @step_result                                -- ← context grows
END
```

Two problems compound each other:

1. **Output grows per step** — each call produces hundreds of lines of code, taking 10–100× longer than a short description.
2. **Context grows per iteration** — `@results` accumulates all prior code and is passed back into every subsequent call. By step 3, the LLM is reading thousands of tokens of prior output before generating thousands more.

The workflow appeared to hang. Each step took several minutes on `qwen2.5-coder`.

**Fix:** Never generate large artifacts inside a WHILE loop. Loop bodies should produce only lightweight text — a decision, a filename, a status word.

---

### Anti-pattern 2: one giant call to generate everything at once

**What went wrong (v1):**

After moving code generation outside the loop, the recipe collected design notes across all steps and fed them into a single `synthesize` call:

```spl
-- After the loop:
GENERATE synthesize(@task, @results) INTO @final_report   -- ← asks for ALL files in one response
CALL write_code_files(@final_report, @output_dir)
```

This worked for small outputs but failed for any task that produced more than 2–3 files. Small/fast models like `qwen2.5-coder` have a limited output token window. When the response was truncated mid-file, the closing ` ``` ` fence was never written, so `write_code_files` could not parse the block. Result: only the first 1–2 files landed on disk with no error — a silent data loss.

**Fix:** Never ask the LLM to emit an unbounded number of large artifacts in a single call. You cannot predict how much output will be generated, and truncation is silent.

---

### Solution (v2 — current): two-phase loops with constant context

The correct pattern mirrors the planning loop itself — use a second WHILE loop to generate artifacts one at a time:

```spl
-- Phase A: outline what needs to be built (tiny output — filenames only)
GENERATE outline_files(@task, @results) INTO @file_outline
GENERATE count_files(@file_outline) INTO @file_count

-- Phase B: generate each file individually (constant context, bounded output)
@file_index := 0
WHILE @file_index < @file_count DO
    GENERATE extract_file(@file_outline, @file_index) INTO @current_file
    GENERATE generate_file(@task, @results, @current_file) INTO @file_code   -- ← ONE file
    CALL write_code_files(@file_code, @output_dir) INTO @file_written         -- ← written immediately
    @file_index := @file_index + 1
END
```

Why this works:

- `@results` (design notes) is **read-only** context — it never grows between iterations.
- Each `generate_file` call produces exactly one file — always within the model's output window.
- Files are written to disk immediately — no accumulation, no silent loss on truncation.
- The same `count / extract / process` idiom is reused from the planning loop — consistent, learnable.

---

### SPL WHILE loop checklist

Before writing a WHILE loop in SPL, ask these questions:

| Question | If yes → action |
|---|---|
| Does the loop variable fed back into the prompt grow each iteration? | Move accumulation outside the loop or use read-only context |
| Does the loop body GENERATE more than a sentence or two? | Move that generation outside the loop (before or after) |
| Does a single post-loop call generate an unbounded number of artifacts? | Replace with a second WHILE loop, one artifact per call |
| Is there a maximum bound on iterations? | Add a `max_N` cap and pass it to `count_*` functions |

**The mental model:** a WHILE loop in SPL is a *router*, not a *generator*. Its job is to decide what to do next (extract, validate, branch) — not to produce the final output. Heavy generation belongs outside loops, in dedicated phases before or after.

## Model selection and output quality

The framework (planning loop → file-by-file generation) is model-agnostic, but **output quality varies significantly**:

| Model | Speed | Code coherence | Recommended for |
|---|---|---|---|
| `qwen2.5-coder` (ollama) | Fast | Poor on multi-file tasks | Quick iteration, testing the workflow |
| `claude-sonnet-4-6` (claude_cli) | Moderate | Excellent | Production-quality output |

### Observed failure mode with small models (`qwen2.5-coder`)

Running with `qwen2.5-coder` produced 4 files but they were **not runnable together**:

- `models.py` used **Flask + SQLAlchemy** (from early design notes)
- `main.py` and `app/main.py` used **FastAPI + in-memory dict** (generated later, independently)
- Two separate `main.py` files existed — the LLM generated both during a replan cycle
- `requirements.txt` listed `sqlite3` (stdlib, not pip-installable) and omitted `fastapi`, `uvicorn`, `pydantic`
- `models.py` called `db.create_all()` at module level — raises `RuntimeError` before app context exists
- Neither `main.py` imported `models.py` — the SQLAlchemy models were dead code

**Root cause:** `qwen2.5-coder` drifted between framework choices across the replanning loop. Each file was generated in isolation without awareness of what the other files used. A stronger model maintains consistency across all files because it better tracks the overall design intent throughout the workflow.

### Recommended command (sonnet 4.6)

```bash
spl-go run cookbook/12_plan_and_execute/plan_execute.spl \
    --adapter claude_cli --model claude-sonnet-4-6 \
    --tools cookbook/12_plan_and_execute/tools.py \
    task="Build a REST API for a todo app" \
    output_dir="cookbook/12_plan_and_execute/output" \
    max_steps=4 \
    2>&1 | tee cookbook/out/12_plan_and_execute-claude-$(date +%Y%m%d_%H%M%S).md
```

## End-to-end demo: from generated API to working UI

After the SPL recipe generates and fixes the backend, two frontend apps were hand-crafted to demonstrate the full stack. Both connect to the same FastAPI backend.

### Step 1 — start the backend

```bash
cd cookbook/12_plan_and_execute/output-claude-sonnet4-6
pip install -r requirements.txt
uvicorn main:app --reload
# API running at http://localhost:8000
# Swagger UI at http://localhost:8000/docs
```

---

### Step 2a — Simple frontend (vanilla HTML/JS)

**No install, no build step** — a single self-contained HTML file.

```bash
conda activate spl
cd ~/projects/digital-duck/SPL20
python3 -m http.server 3001 \
    --directory cookbook/12_plan_and_execute/front-end-simple
# open http://localhost:3001
```

**What it teaches:**

| Concept | Where |
|---------|-------|
| `fetch()` + async/await | `apiFetch()` helper and every handler |
| DOM manipulation | `render()` builds the list from scratch on each load |
| Event listeners | form submit, filter buttons, checkbox, delete |
| Error handling | try/catch + toast notification |
| HTML escaping | `escHtml()` — prevents XSS from user input |

Good starting point — all logic in one file, readable top to bottom.

---

### Step 2b — Vue 3 frontend (component-based)

**Requires Node.js.** Uses Vite as the dev server; no CORS config needed (Vite proxies `/api` → `http://localhost:8000`).

```bash
cd cookbook/12_plan_and_execute/front-end-vuejs
npm install
npm run dev
# open http://localhost:5173
```

**Component layout:**

```
App.vue               ← owns all state (todos, filter, error)
  ├── TodoInput.vue   ← "add todo" form   (emits: add)
  ├── TodoFilter.vue  ← All/Active/Done   (emits: change)
  └── TodoList.vue    ← renders the list
        └── TodoItem.vue  ← one row       (emits: toggle, delete)
```

**Key Vue 3 concepts demonstrated (with inline comments in each file):**

| Concept | File | What it does |
|---------|------|-------------|
| `ref()` | `App.vue` | Reactive value — UI re-renders on change |
| `computed()` | `App.vue` | Derived state (filtered list, counts) |
| `onMounted()` | `App.vue` | Fetch todos when the app first loads |
| `defineProps` | all components | Receive data from the parent |
| `defineEmits` | all components | Send events up to the parent |
| `v-model` | `TodoInput.vue` | Two-way binding between input and `ref` |
| `v-for` | `TodoList.vue`, `TodoFilter.vue` | Render a list |
| `v-if / v-else` | `TodoList.vue` | Conditional rendering |
| `:class` | `TodoItem.vue` | Apply CSS class conditionally |
| `$emit` | child components | Bubble event up to parent |

**Data flow — "props down, events up":**

```
App.vue (state lives here)
  │ props down                events up │
  ▼                                     ▲
TodoInput / TodoFilter / TodoList ──► App.vue calls API → reloads state
```

State lives in one place; children only display and report user actions.

---

## Tools

`tools.py` provides `write_code_files(content, output_dir)` — parses fenced code blocks with `# filename: path` as the first line and writes each to `output_dir`. Loaded with `--tools cookbook/12_plan_and_execute/tools.py`.
