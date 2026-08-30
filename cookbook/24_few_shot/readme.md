# Recipe 24 — Few-Shot Prompting

Embed gold-standard examples in `SELECT` context to guide output format and style — no fine-tuning required.

## Key Feature

`CREATE FUNCTION` returns domain-specific examples as a `TEXT` value. The `SELECT` includes them as `examples` alongside the input, and `GENERATE` uses both to produce consistently formatted output.

## Usage

```bash
# General sentiment + summary
spl-go run cookbook/24_few_shot/few_shot.spl --adapter ollama --model gemma3 \
    text="The quarterly results exceeded all analyst forecasts by a significant margin"

# Finance domain
spl-go run cookbook/24_few_shot/few_shot.spl --adapter ollama \
    text="Revenue grew 12% YoY despite headwinds in APAC" domain="finance"

# Ops/incident triage
spl-go run cookbook/24_few_shot/few_shot.spl --adapter ollama \
    text="System outage detected in EU-WEST-2 affecting 15% of users" domain="ops"
```

## Why few-shot in SELECT

- Examples are version-controlled with the `.spl` file
- Domain parameter makes the example set swappable without touching the prompt logic
- Same pattern works for any output format: JSON, YAML, markdown tables, code
