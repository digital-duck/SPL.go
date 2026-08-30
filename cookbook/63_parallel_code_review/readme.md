# Recipe 63: Parallel Code Review

**Category:** agentic / SPL 3.0  
**SPL version:** 3.0  
**Type:** TEXT → TEXT  
**LLM required:** Yes — any text model (default: `gemma4`)  
**Demonstrates:** `IMPORT`, `CALL PARALLEL`, heterogeneous fan-out + merge

---

## What it does

Runs three independent quality checks on a code snippet **concurrently**, then
merges the results into a single prioritised action plan.

```
@code ──┬── style_review    ──► @style_fb  ──┐
        ├── security_audit  ──► @sec_fb    ──┼── merge ──► @report
        └── test_generator  ──► @test_fb   ──┘
```

The three reviewers are **heterogeneous** — each is its own imported sub-workflow
with its own prompt and output budget — but they all take the same `@code` input.
Because they don't depend on each other's output, they run in parallel.

---

## Files

| File | Role |
|---|---|
| `parallel_code_review.spl` | Main orchestrator — imports sub-workflows, fans out, merges |
| `00_style_review.spl` | Sub-workflow: style, readability, correctness |
| `01_security_audit.spl` | Sub-workflow: security vulnerability scan |
| `02_test_generator.spl` | Sub-workflow: unit test case generation |
| `logs-spl/` | Execution logs (auto-created on run) |

---

## Prerequisites

```bash
ollama serve
ollama pull gemma4      # or llama3.2, qwen3, deepseek-r1, etc.
```

No API keys required for local runs.

---

## Running

```bash
# Inline code snippet
spl-go run cookbook/63_parallel_code_review/parallel_code_review.spl \
    --param code="def add(a, b): return a - b" \
    --param review_model="gemma3"

# From a file
spl-go run cookbook/63_parallel_code_review/parallel_code_review.spl \
    --param code="$(cat my_module.py)" lang=python

# Go code
spl-go run cookbook/63_parallel_code_review/parallel_code_review.spl \
    --param code="$(cat main.go)" lang=go model=gemma4

# With spl-go
spl-go run cookbook/63_parallel_code_review/parallel_code_review.spl \
    --param code="def add(a, b): return a - b" --adapter ollama -m gemma4

# Dry-run
spl-go run cookbook/63_parallel_code_review/parallel_code_review.spl \
    --param code="def add(a, b): return a - b" --adapter echo
```

---

## Expected output

```
============================================================
Workflow Status: complete
LLM Calls: 4 | Tokens: ~1200 in / ~600 out
Latency: ~4000ms (parallel branches) | Cost: $0.000000
------------------------------------------------------------
Committed Output:
## Action Items
1. [CRITICAL] Function name `add` returns `a - b` — subtraction, not addition. ...
2. [LOW] No input validation for non-numeric types.

## Test Coverage
```python
import pytest
from module import add

def test_add_positive():
    assert add(2, 3) == 5
...
```

## Summary
The code contains a critical logic error ...
============================================================
```

LLM calls breakdown: 3 parallel sub-workflow calls + 1 serial merge.

---

## SPL 3.0 concepts illustrated

| Concept | Where |
|---|---|
| `IMPORT 'file'` | Load 3 sub-workflow definitions before execution |
| `CALL PARALLEL ... END` | Fan-out to `style_review`, `security_audit`, `test_generator` |
| Heterogeneous branches | Each branch has its own prompt, output budget, and focus |
| Branch isolation | All branches read `@code`; each writes only to its own `INTO @var` |
| Named arguments in `CALL` | `lang=@lang, model=@model, log_dir=@log_dir` |
| Fan-out → merge | 3 parallel outputs feed 1 final `GENERATE merge_reviews(...)` |

---

## LLM-compiled targets (recipe 65)

Generated via `cookbook/65_llm_splc/` using `--adapter claude_cli`. See recipe 65 readme for commands.

### AutoGen (`targets/autogen/parallel_code_review.py`)

**Generated:** 2026-04-20 | 3 LLM calls | 316s | 9610 chars

**What the compiler got right:**
- `AssistantAgent + UserProxyAgent` pattern with per-agent `llm_config`
- `asyncio.gather(*tasks)` for `CALL PARALLEL` — three reviews dispatched concurrently
- Ollama-compatible config (`base_url: http://localhost:11434/v1`)
- Per-agent `max_tokens` matching SPL `WITH OUTPUT BUDGET` values
- All verbatim prompts from `CREATE FUNCTION` blocks preserved
- Logging messages mirror SPL `LOGGING` statements
- Exception handling maps to both `EXCEPTION WHEN` blocks

**Known gap — `initiate_chat` is synchronous:**
```python
# Generated (blocks event loop):
chat_result = self.user_proxy.initiate_chat(agent, message=message, max_turns=1)

# Should be (true async):
chat_result = await asyncio.to_thread(
    self.user_proxy.initiate_chat, agent, message=message, max_turns=1
)
```
The three `asyncio.gather` tasks appear parallel but run sequentially because `initiate_chat` blocks the event loop. Fix before production use.

**Self-review caught:** default model mismatch (`gemma3` → `gemma4` to match SPL spec). Fix step corrected it.

---

### CrewAI (`targets/crewai/parallel_code_review.py`)

**Generated:** 2026-04-20 (draft — re-run pending fix step validation)

**What the compiler got right:**
- `Agent + Task + Crew + Process` pattern — correct CrewAI idioms
- `Process.parallel` for the `CALL PARALLEL` branches
- Two-crew pattern: parallel review crew → merge crew
- `gemma3` as default model (Ollama-compatible)
- `merge_reviews` prompt verbatim from SPL `CREATE FUNCTION`
- Exception handling maps to both `EXCEPTION WHEN` blocks

**Known gap:**
- `langchain_community.llms.Ollama` is deprecated; use `langchain_ollama` or direct Ollama client
- `parallel_results[0/1/2]` — CrewAI `kickoff()` returns `CrewOutput`, not a list; use `.tasks_output[i].raw`
