# Recipe 13: Map-Reduce Summarizer

Handles documents too large for a single context window by splitting into chunks (map), summarizing each independently, then combining into a final summary (reduce).

## Pattern

```
chunk_plan(document) → N chunks
  └─► MAP: summarize_chunk(chunk_i) for i in 0..N
        └─► REDUCE: reduce_summaries(all_summaries, style)
              └─► quality_score → refine if score ≤ 0.7
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `document` | TEXT | *(required)* | Full document text to summarize |
| `style` | TEXT | *(required)* | Output style (e.g. `bullet points`, `executive brief`, `narrative`) |

## Sample Files

| File | Description | Size |
|------|-------------|------|
| `large_doc.txt` | 6-section technical report on distributed AI infrastructure (~2,800 words) | multi-chunk |
| `sample.md` | SPL 3.0 overview document in Markdown (~800 words) | single/two-chunk |

## Usage

```bash
# Long document — exercises chunking across multiple sections
spl-go run cookbook/13_map_reduce/map_reduce.spl --adapter ollama \
    --model gemma3 --tools cookbook/13_map_reduce/tools.py \
    document="$(cat cookbook/13_map_reduce/large_doc.txt)" \
    style="bullet points" \
    2>&1 | tee cookbook/out/13_map_reduce-long-$(date +%Y%m%d_%H%M%S).md

# Medium document — Markdown format, tests section boundary handling
spl-go run cookbook/13_map_reduce/map_reduce.spl --adapter ollama \
    document="$(cat cookbook/13_map_reduce/sample.md)" \
    style="executive brief" \
    2>&1 | tee cookbook/out/13_map_reduce-medium-$(date +%Y%m%d_%H%M%S).md

# Short inline text — single-chunk path
spl-go run cookbook/13_map_reduce/map_reduce.spl --adapter ollama \
    document="Artificial intelligence has transformed how organizations process and analyze large volumes of unstructured data. Key applications include document summarization, sentiment analysis, and automated report generation." \
    style="narrative" \
    2>&1 | tee cookbook/out/13_map_reduce-short-$(date +%Y%m%d_%H%M%S).md
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | Quality score > 0.7, committed directly |
| `refined` | Quality was low, one refinement pass applied |
| `partial` | Context too long even for chunking |
| `budget_limit` | Token budget exceeded during reduce |
