
recipe suggested by Qwen 3.6 Plus

```bash
spl-go run cookbook/48_credit_risk/assess_credit_risk.spl \
  --adapter ollama --model gemma3 \
  applicant_data="$(cat cookbook/48_credit_risk/data/applicant_680.json)" \
  credit_score=680 \
  --tools cookbook/tools/finance_helpers.py


# Test 1: Auto-reject path (score < 600)
spl-go run cookbook/48_credit_risk/assess_credit_risk.spl \
  --adapter ollama --model gemma3 \
  applicant_data="$(cat cookbook/48_credit_risk/data/applicant_680.json)" \
  credit_score=550 \
  --tools cookbook/tools/finance_helpers.py

# REJECTED

# Test 2: Auto-approve path (score >= 750)
spl-go run cookbook/48_credit_risk/assess_credit_risk.spl \
  --adapter ollama --model gemma3 \
  applicant_data="$(cat cookbook/48_credit_risk/data/applicant_680.json)" \
  credit_score=780 \
  --tools cookbook/tools/finance_helpers.py

# Test 3: Edge case - exactly 600
spl-go run cookbook/48_credit_risk/assess_credit_risk.spl \
  --adapter ollama --model gemma3 \
  applicant_data="$(cat cookbook/48_credit_risk/data/applicant_680.json)" \
  credit_score=600 \
  --tools cookbook/tools/finance_helpers.py  
```