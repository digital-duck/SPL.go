# Recipe 15: Automated Code Review

Multi-pass code review: security audit, performance analysis, style check, and bug detection — all scored and synthesized into a structured verdict.

## Pattern

```
security_audit  ──┐
performance_review─┤ → severity scores
style_review    ──┤        └─► synthesize_review → EVALUATE sec_score
bug_detection   ──┘                  ├─ > 8 → block
                                     ├─ > 5 → request_changes
                                     └─ else → approve
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `code` | TEXT | *(required)* | Source code to review |
| `language` | TEXT | *(required)* | Programming language (e.g. Python, Go, TypeScript) |

## Usage

```bash
spl-go run cookbook/15_code_review/code_review.spl --adapter ollama \
    code="$(cat digest.go)" \
    language="Go" \
    2>&1 | tee cookbook/out/15_code_review-$(date +%Y%m%d_%H%M%S).md


spl-go run cookbook/15_code_review/code_review.spl --adapter ollama \
    code="def foo(x): return eval(x)" \
    language="Python"

spl-go run cookbook/15_code_review/code_review.spl --adapter ollama \
    code="$(cat main.py)" \
    language="Python" \
    2>&1 | tee cookbook/out/15_code_review-python-$(date +%Y%m%d_%H%M%S).md


```

## Output status

| Status | Verdict |
|---|---|
| `critical_issues` | `block` — security score > 8 |
| `needs_fixes` | `request_changes` — security score > 5 |
| `approved` | `approve` — no significant issues |
| `partial_large_file` | File too large; quick review on summary |
| `security_only` | Budget exceeded after security pass |
