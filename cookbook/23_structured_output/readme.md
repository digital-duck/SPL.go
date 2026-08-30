# Recipe 23 — Structured Output

Extract typed, schema-validated data from free-form text using `CREATE FUNCTION` with a JSON schema.

## Key Feature

`CREATE FUNCTION` defines a reusable schema that constrains LLM output to valid JSON. The model must return data matching the schema — no post-processing guesswork.

## Usage

```bash
# Extract person + org data
spl-go run cookbook/23_structured_output/structured_output.spl --adapter ollama --model gemma3 \
    text="John Smith, 42, joined Acme Corp in March 2021 earning \$95,000/year"

# Extract invoice fields
spl-go run cookbook/23_structured_output/structured_output.spl --adapter ollama \
    text="Invoice #INV-2045 from TechSupplies Ltd dated 15 Jan 2024 for \$3,420.50 due in 30 days"
```

## Expected Output

```json
{
  "people": [
    { "name": "John Smith", "age": 42, "salary": 95000, "start_date": "2021-03-01" }
  ],
  "organizations": [
    { "name": "Acme Corp", "type": "employer" }
  ],
  "amounts": [
    { "value": 95000, "currency": "USD", "label": "annual salary" }
  ],
  "dates": [
    { "value": "2021-03-01", "label": "start date" }
  ]
}
```

## Why `CREATE FUNCTION` for schemas

- Schema is defined once, reused across multiple `PROMPT` blocks
- Schema version is tracked with the `.spl` file — no separate config
- Works with any adapter that supports structured/constrained output
