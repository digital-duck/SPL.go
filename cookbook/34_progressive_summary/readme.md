# Recipe 34: Progressive Summarizer

Produces layered summaries at increasing granularity: one sentence → one paragraph → one page. Each layer builds on the previous for coherence, then fidelity is verified against the original.

## Pattern

```
summarize(text, sentence constraint)   → @sentence_summary
  └─► expand_summary(sentence → paragraph) → @paragraph_summary
        └─► expand_summary(paragraph → page) → @page_summary (if layers ≥ 3)
              └─► verify_summary_fidelity → assemble_summary_package
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `text` | TEXT | *(required)* | Full text to summarize |
| `audience` | TEXT | `general` | Target audience (e.g. `general`, `executive`, `technical`) |
| `layers` | INT | `3` | Number of summary layers (1=sentence only, 2=+paragraph, 3=+page) |

## Usage

```bash
spl-go run cookbook/34_progressive_summary/progressive_summary.spl --adapter ollama \
    text="$(cat long_article.txt)"

spl-go run cookbook/34_progressive_summary/progressive_summary.spl --adapter ollama \
    text="$(cat research_paper.txt)" \
    audience="executive" \
    layers=3

# Inline text
spl-go run cookbook/34_progressive_summary/progressive_summary.spl --adapter ollama \
    text="Artificial intelligence has transformed industries..." \
    audience="technical"
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | All layers generated with fidelity check |
| `complete_chunked` | Text was too long; chunked before summarizing |
