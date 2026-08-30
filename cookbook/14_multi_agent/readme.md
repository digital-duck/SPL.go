# Recipe 14: Multi-Agent Collaboration

Three specialized agents collaborate via `CALL`: a Researcher gathers facts, an Analyst identifies trends and risks, a Writer produces the polished report.

## Pattern

```
CALL researcher(@topic)            → @research
  └─► CALL analyst(@research, @topic)  → @analysis
        └─► CALL writer(@research, @analysis, @topic) → @report
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `topic` | TEXT | *(required)* | The subject for the collaborative report |

## Usage

```bash
spl-go run cookbook/14_multi_agent/multi_agent.spl --adapter ollama \
    topic="Future of renewable energy" \
    2>&1 | tee cookbook/out/14_multi_agent-$(date +%Y%m%d_%H%M%S).md

spl-go run cookbook/14_multi_agent/multi_agent.spl --adapter ollama \
    topic="Impact of AI on healthcare"

spl-go run cookbook/14_multi_agent/multi_agent.spl --adapter ollama --model llama3.2 \
    topic="State of quantum computing in 2025"
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | All three agents finished successfully |
| `partial_research_only` | Budget exceeded after research phase |
| `restarted` | Hallucination detected; retried at lower temperature |
