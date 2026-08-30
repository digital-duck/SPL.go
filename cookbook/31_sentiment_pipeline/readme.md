# Recipe 31: Sentiment Pipeline

Batch sentiment analysis over a list of items. Classifies each item with a label, score,
confidence, emotions, and key phrases; computes aggregate statistics; identifies extremes;
and produces a trend narrative report.

## What's in this recipe

| File | Purpose |
|---|---|
| `sentiment.spl` | Main SPL workflow |
| `tools.py` | Python tools: `load_items`, `split_items`, `compute_stats`, `find_extremes` |
| `reviews/product_reviews.txt` | 15 product reviews (positive, negative, mixed, neutral) |
| `reviews/support_tickets.txt` | 12 customer support ticket excerpts |
| `reviews/social_media.txt` | 15 social media posts |

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `filename` | TEXT | `''` | File in `reviews/` to load (e.g. `product_reviews.txt`) |
| `items` | TEXT | `''` | Inline delimited text (alternative to `filename`) |
| `delimiter` | TEXT | `\n` | Separator between items (`\n`, `\|`, `,`, etc.) |
| `domain` | TEXT | `general` | Context hint for the LLM: `product_reviews`, `support_tickets`, `social_media` |

Pass either `filename` **or** `items` — if both are given, `filename` takes precedence.

## Usage

Always pass `--tools tools.py`:

```bash
# Product reviews → full report
spl-go run cookbook/31_sentiment_pipeline/sentiment.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/31_sentiment_pipeline/tools.py \
    filename=product_reviews.txt \
    domain=product_reviews \
    2>&1 | tee cookbook/out/31_sentiment-$(date +%Y%m%d_%H%M%S).md

# Support tickets
spl-go run cookbook/31_sentiment_pipeline/sentiment.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/31_sentiment_pipeline/tools.py \
    filename=support_tickets.txt \
    domain=support_tickets

# Social media posts
spl-go run cookbook/31_sentiment_pipeline/sentiment.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/31_sentiment_pipeline/tools.py \
    filename=social_media.txt \
    domain=social_media

# Inline pipe-delimited (no file needed)
spl-go run cookbook/31_sentiment_pipeline/sentiment.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/31_sentiment_pipeline/tools.py \
    items="Great product, love it! | Terrible experience, never again | It was okay I guess" \
    delimiter="|" \
    domain=product_reviews
```

## Workflow steps

```
filename / items
    │
    ├─ CALL load_items()         ← reads file from reviews/, zero LLM cost
    ├─ CALL split_items()        ← splits by delimiter → JSON array, zero LLM cost
    │
    ├─ GENERATE batch_sentiment()  ← LLM: classify every item per schema
    │
    ├─ CALL compute_stats()      ← label counts, score stats, confidence, ratio, zero LLM cost
    ├─ CALL find_extremes()      ← most positive, most negative, lowest confidence, zero LLM cost
    │
    ├─ GENERATE summarize_sentiment_trends()  ← LLM: narrative from stats + extremes
    └─ GENERATE assemble_sentiment_report()   ← LLM: final formatted report

    EXCEPTION ContextLengthExceeded
        → CALL compute_stats() + CALL find_extremes()
        → GENERATE summarize_sentiment_trends()  → COMMIT stats_only
```

## Python tools (`tools.py`)

### `load_items(filename)`
Loads a `.txt` file from the `reviews/` folder. Returns raw text or an error listing available files.

### `split_items(file_content, inline_items, delimiter)`
Splits whichever source is non-empty by the given delimiter, strips whitespace, drops blank lines,
and returns a JSON array of strings. Handles the literal `\n` escape automatically.

### `compute_stats(sentiment_json)`
Parses the JSON array from `batch_sentiment` and computes:

| Field | Description |
|---|---|
| `total` | Total items analysed |
| `label_counts` | Count per label (positive / negative / neutral / mixed) |
| `label_pct` | Percentage per label |
| `score.mean/min/max/std_dev` | Score statistics across all items |
| `confidence.mean/min/max` | Confidence statistics |
| `sentiment_ratio` | positive_count ÷ negative_count (> 1 = net positive) |

### `find_extremes(sentiment_json)`
Deterministically locates notable items:
- `most_positive` — highest score
- `most_negative` — lowest score
- `lowest_confidence` — least certain classification
- `all_mixed` — all items labelled `mixed`
- `all_negative` — all items labelled `negative`

## Sentiment schema per item

| Field | Type | Description |
|---|---|---|
| `item` | string | Original text (echoed back for traceability) |
| `label` | enum | `positive`, `negative`, `neutral`, `mixed` |
| `score` | float | −1.0 (most negative) to +1.0 (most positive) |
| `confidence` | float | 0.0 to 1.0 — model certainty |
| `emotions` | array | e.g. `joy`, `anger`, `frustration`, `surprise` |
| `key_phrases` | array | Most sentiment-bearing phrases |

## Sample data

| File | Items | Domain | Notable scenarios |
|---|---|---|---|
| `product_reviews.txt` | 15 | product_reviews | Crashes, billing issues, onboarding praise, mobile complaints, mixed overall |
| `support_tickets.txt` | 12 | support_tickets | Angry churn threats, grateful resolutions, URGENT escalation, cautious stay |
| `social_media.txt` | 15 | social_media | Hype posts, outage complaints, sarcasm, feature excitement, billing frustration |

## Output status

| Status | Meaning |
|---|---|
| `complete` | Full report with all items analysed and trend narrative |
| `stats_only` | Too many items for full report — statistics and trend summary returned |
