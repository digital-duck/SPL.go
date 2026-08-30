# Recipe 09: Chain of Thought

Multi-step reasoning pipeline: Research → Analyze → Summarize. Each step feeds its output into the next, building progressively refined understanding.

## Pattern

```
research(topic) → @research
  └─► analyze(@research) → @analysis
        └─► summarize(@analysis) → @summary
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `topic` | TEXT | *(required)* | The topic to research, analyze, and summarize |

## Usage

```bash
spl-go run cookbook/09_chain_of_thought/chain.spl --adapter ollama \
    topic="distributed AI inference"

spl-go run cookbook/09_chain_of_thought/chain.spl --adapter ollama --model llama3.2 \
    topic="quantum computing"

spl-go run cookbook/09_chain_of_thought/chain.spl --adapter ollama \
    topic="the history of the microprocessor" \
    2>&1 | tee cookbook/out/09_chain_of_thought-$(date +%Y%m%d_%H%M%S).md 
# /home/gongai/projects/digital-duck/SPL20/cookbook/out/run_all_20260320_192010.md
```
