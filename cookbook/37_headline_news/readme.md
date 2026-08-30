# Recipe 37: Headline News Aggregator

Generate, expand, and evaluate a daily news digest for any topic area. Uses the `claude_cli` adapter with the Opus model for broad knowledge synthesis and nuanced coverage evaluation.

## Pattern

```
generate_headlines(topic, max_headlines) → headlines
  └─► expand_headlines(headlines) → expanded summaries
        └─► evaluate_coverage(expanded) → coverage_score
              ├─► score > 0.75 → format_digest → COMMIT (complete)
              └─► score ≤ 0.75 → fill_coverage_gaps → format_digest → COMMIT (refined)
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `topic` | TEXT | *(required)* | Topic area to cover (e.g. `artificial intelligence`, `climate policy`) |
| `date` | TEXT | `today` | Date reference for the digest (e.g. `2026-03-21`, `this week`) |
| `max_headlines` | INTEGER | `7` | Maximum number of headlines to generate |
| `format` | TEXT | `structured` | Output format: `structured`, `executive brief`, `bullet points`, `narrative` |
| `perspective` | TEXT | `balanced` | Coverage angle: `balanced`, `technical`, `business`, `global`, `policy` |

## Usage

```bash
spl-go run cookbook/37_headline_news/headline_news.spl --adapter ollama --model phi4 \
    topic="artificial intelligence" \
    2>&1 | tee cookbook/out/37_headline_news-ai-ollama_phi4-$(date +%Y%m%d_%H%M%S).md


# Default — balanced structured digest on AI
spl-go run cookbook/37_headline_news/headline_news.spl --adapter claude_cli --model claude-opus-4-6 \
    topic="artificial intelligence" \
    2>&1 | tee cookbook/out/37_headline_news-ai-$(date +%Y%m%d_%H%M%S).md

# Executive brief on renewable energy
spl-go run cookbook/37_headline_news/headline_news.spl --adapter claude_cli --model claude-opus-4-6 \
    topic="renewable energy" \
    format="executive brief" \
    max_headlines=5 \
    2>&1 | tee cookbook/out/37_headline_news-energy-$(date +%Y%m%d_%H%M%S).md

# Global perspective on quantum computing
spl-go run cookbook/37_headline_news/headline_news.spl --adapter claude_cli --model claude-opus-4-6 \
    topic="quantum computing" \
    format="bullet points" \
    perspective="global" \
    2>&1 | tee cookbook/out/37_headline_news-quantum-$(date +%Y%m%d_%H%M%S).md

# Policy angle on semiconductor supply chain
spl-go run cookbook/37_headline_news/headline_news.spl --adapter claude_cli --model claude-opus-4-6 \
    topic="semiconductor supply chain" \
    format="narrative" \
    perspective="policy" \
    2>&1 | tee cookbook/out/37_headline_news-semicon-$(date +%Y%m%d_%H%M%S).md
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | Coverage score > 0.75, digest committed directly |
| `refined` | Coverage gaps detected, one gap-fill pass applied |
| `partial` | Context too long, headlines-only digest committed |
| `budget_limit` | Token budget exceeded, expanded summaries committed as-is |

## Design Notes

**Why Opus?**
The coverage evaluation step (`evaluate_coverage`) requires genuine judgment about whether major angles of a topic are represented. Opus has broader knowledge and better synthesis capability for this task compared to smaller models.

**Knowledge cutoff awareness**
This recipe synthesizes what the model knows about recent developments. It is not a live news feed — results reflect the model's training data. For genuinely real-time news, extend this recipe by:
- Adding a `tool_use` call (Recipe 36 pattern) to a search API (DuckDuckGo, NewsAPI, Brave Search)
- Using the `gemini` adapter which has Google Search grounding built in

**Coverage evaluation**
The `EVALUATE @coverage_score` step is a semantic branch — the score is produced by an LLM judge, not a deterministic function. This means the `refined` vs `complete` path is non-deterministic: the same topic may take different paths across runs. This is expected behavior (SPL 3.0 semantic termination).

## Extending This Recipe

| Extension | How |
|---|---|
| Real-time news | Add `CALL search_web(@topic)` via tool_use (Recipe 36 pattern) before Step 1 |
| Gemini grounding | Replace `--adapter claude_cli` with `--adapter gemini` for Google Search-grounded output |
| Multi-topic digest | Wrap in a `FOR` loop over a list of topics (Recipe 34 Progressive Summary pattern) |
| Scheduled daily run | Wrap in a cron job or CI pipeline; pipe output to Slack/email |
| Comparison across dates | Run with `date="2026-03-21"` and `date="2026-03-14"` and diff the digests |
