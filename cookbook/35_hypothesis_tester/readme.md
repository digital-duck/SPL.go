# Recipe 35: Hypothesis Tester

Scientific reasoning pipeline: formulate H0/H1 hypotheses from an observation, design a test plan, evaluate evidence, and draw a conclusion based on a confidence threshold.

## Pattern

```
formulate_hypotheses(observation) → @hypotheses
  └─► design_test(@hypotheses) → @test_plan
        └─► evaluate_evidence → @evidence_json
              └─► extract_confidence → @confidence
                    ├─ ≥ threshold  → COMMIT concluded / h1_supported
                    ├─ ≥ 0.4        → COMMIT inconclusive / needs_more_data
                    └─ < 0.4        → COMMIT concluded / h0_not_rejected
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `observation` | TEXT | *(required)* | The empirical observation or claim to test |
| `domain` | TEXT | `general` | Domain context (e.g. `product`, `management`, `science`) |
| `confidence_threshold` | FLOAT | `0.7` | Minimum confidence to support H1 |

## Usage

```bash
spl-go run cookbook/35_hypothesis_tester/hypothesis.spl --adapter ollama \
    observation="Remote teams show lower productivity in the first month after joining"

spl-go run cookbook/35_hypothesis_tester/hypothesis.spl --adapter ollama \
    observation="Users who receive onboarding emails churn 30% less" \
    domain="product" \
    confidence_threshold=0.75

spl-go run cookbook/35_hypothesis_tester/hypothesis.spl --adapter ollama --model llama3.2 \
    observation="Daily standups reduce sprint velocity by 10%" \
    domain="management"
```

## Output status

| Status | Verdict | Meaning |
|---|---|---|
| `concluded` | `h1_supported` | Evidence supports the alternative hypothesis |
| `inconclusive` | `needs_more_data` | Confidence between 0.4 and threshold |
| `concluded` | `h0_not_rejected` | Evidence too weak to reject null hypothesis |
| `hypotheses_only` | — | Evidence evaluation failed |
