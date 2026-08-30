# Recipe 43: Prompt Self-Tuning (Meta-Programming)

## Description
This recipe demonstrates how SPL can be used for "meta-prompting"—using the language to improve its own performance. It takes a "failed" example and a baseline prompt, generates variations, and then runs a mini-A/B test to select the best one based on a quality check.

## Key Features
- **Meta-Programming:** Uses an LLM to generate variations of its own instructions.
- **Automated A/B Testing:** Simulates testing multiple prompts on the same input.
- **Data-Driven Prompt Engineering:** Selection of the winner is based on a deterministic quality metric.

## Usage
```bash
spl-go run cookbook-v3.0/43_prompt_self_tuning/prompt_self_tuning.spl --tools cookbook-v3.0/tools.py
```
