# Recipe 27: Data Extraction

Two implementations showing the evolution from SPL 1.0 PROMPT to SPL 3.0 WORKFLOW with MAP configuration.

| File | Style | Pattern |
|---|---|---|
| `data_extraction.spl` | SPL 1.0 PROMPT | Schema-constrained extraction |
| `data_extraction_map.spl` | SPL 3.0 WORKFLOW | MAP config lookup + result annotation |

---

## 27a — data_extraction.spl (SPL 1.0 PROMPT)

Pulls structured fields from messy free-form text using schema-constrained extraction. Supports three formats: `general`, `invoice`, and `contract`.

### Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `text` | TEXT | *(required)* | The raw text to extract fields from |
| `format` | TEXT | `general` | Schema type: `general`, `invoice`, or `contract` |

### Usage

```bash
# General extraction (names, dates, amounts, references)
spl-go run cookbook/27_data_extraction/data_extraction.spl --adapter ollama \
    text="Please process payment of USD 4,250.00 to Riverside Consulting (ref: PO-8821) by end of March."

# Invoice extraction
spl-go run cookbook/27_data_extraction/data_extraction.spl --adapter ollama \
    text="$(cat invoice.txt)" \
    format="invoice"

# Contract extraction
spl-go run cookbook/27_data_extraction/data_extraction.spl --adapter ollama \
    text="$(cat contract.txt)" \
    format="contract"
```

### Schemas

| Format | Extracted fields |
|---|---|
| `general` | names, organizations, dates, amounts, references, locations |
| `invoice` | invoice_number, vendor, amount, currency, issue_date, due_date, line_items, po_reference |
| `contract` | parties, effective_date, expiry_date, value, jurisdiction, key_obligations |

---

## 27b — data_extraction_map.spl (SPL 3.0 WORKFLOW + MAP)

Same extraction task, rewritten as a WORKFLOW using the MAP type for configuration.
Instead of IF/ELSE chains per format, a single MAP routes each format to its fields.
Adding a new format is one line in the MAP — no new branch needed.

### MAP features demonstrated

| Feature | SPL syntax | Purpose |
|---|---|---|
| MAP literal | `@field_map := {'invoice': '...', 'contract': '...'}` | Per-format config table |
| Subscript read | `@fields := @field_map[@format]` | Config lookup — replaces IF/ELSE |
| `map_has()` | `map_has(@field_map, @format)` | Input validation |
| `map_merge()` | `map_merge(@meta, {'data': @extracted})` | Annotate result with metadata |

### Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `text` | TEXT | sample invoice text | The raw text to extract fields from |
| `format` | TEXT | `invoice` | `general`, `invoice`, or `contract` (falls back to `general` if unknown) |
| `model` | TEXT | `gemma3` | LLM model to use |
| `log_dir` | TEXT | `cookbook/27_data_extraction/logs-map` | Output directory |

### Usage

```bash
# Default (invoice format, gemma3)
spl-go run cookbook/27_data_extraction/data_extraction_map.spl --adapter ollama

# General extraction
spl-go run cookbook/27_data_extraction/data_extraction_map.spl --adapter ollama \
    format="general" \
    text="Please process payment of USD 4,250.00 to Riverside Consulting (ref: PO-8821) by end of March."

# Invoice with Claude
spl-go run cookbook/27_data_extraction/data_extraction_map.spl --adapter claude_cli \
    format="invoice" \
    text="Invoice #INV-042 from Acme Corp, USD 3,750.00 due 2024-04-15, PO: PO-9901." \
    model="claude-sonnet-4-6" \
    log_dir="cookbook/27_data_extraction/logs-map2"

# Contract
spl-go run cookbook/27_data_extraction/data_extraction_map.spl --adapter ollama \
    format="contract" \
    text="$(cat contract.txt)"

# Unknown format → auto-falls back to general
spl-go run cookbook/27_data_extraction/data_extraction_map.spl --adapter ollama \
    format="email"
```

### Output

The COMMIT value is a MAP JSON object containing both run metadata and extracted data:

```json
{
  "format": "invoice",
  "model": "gemma3",
  "fields": "invoice_number, vendor, amount, currency, issue_date, due_date, po_reference",
  "data": "{\"invoice_number\": \"INV-2024-0042\", \"vendor\": \"Acme Corp\", ...}"
}
```

Extracted data is also written to `@log_dir/result_<format>.md`.
