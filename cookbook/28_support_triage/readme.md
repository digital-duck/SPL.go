# Recipe 28: Customer Support Triage

Classify → route → draft response in one workflow. Detects urgency, escalates critical tickets
immediately, and drafts a quality-checked response grounded in **real order data** for everything else.

## What's in this recipe

| File | Purpose |
|---|---|
| `support_triage.spl` | Main SPL workflow |
| `tools.py` | Python tools: `extract_order_numbers`, `lookup_order` |
| `orders.json` | Sample order data (8 orders, various statuses) |

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `ticket` | TEXT | *(required)* | The raw customer support message |
| `product` | TEXT | `CloudDash` | Product name to reference in the response |
| `tone` | TEXT | `professional` | Response tone: `professional`, `empathetic`, `formal`, `friendly` |

## Usage

Always pass `--tools tools.py` so the order-lookup tools are available:

```bash
# Billing — duplicate charge on a real order (ORD-12345 exists in orders.json)
spl-go run cookbook/28_support_triage/support_triage.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/28_support_triage/tools.py \
    ticket="My account has been charged twice for the same order #ORD-12345" \
    2>&1 | tee cookbook/out/28_support_triage-$(date +%Y%m%d_%H%M%S).md

# Shipping delay (ORD-67890 shows stuck in Memphis hub)
spl-go run cookbook/28_support_triage/support_triage.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/28_support_triage/tools.py \
    ticket="Package for order ORD-67890 hasn't arrived, it's been 10 days" \
    tone="empathetic"

# Wrong item received (ORD-11111 shows fulfillment mispick)
spl-go run cookbook/28_support_triage/support_triage.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/28_support_triage/tools.py \
    ticket="I received the wrong product for order ORD-11111"

# Login issue — no order number, LLM responds on ticket text alone
spl-go run cookbook/28_support_triage/support_triage.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/28_support_triage/tools.py \
    ticket="I can't log in, password reset email never arrives" \
    tone="friendly"

# Unknown order number — tool tells LLM to ask customer to confirm
spl-go run cookbook/28_support_triage/support_triage.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/28_support_triage/tools.py \
    ticket="Where is my order #99999?"
```

## Workflow steps

```
ticket text
    │
    ├─ CALL extract_order_numbers()  ← deterministic regex, zero LLM cost
    │       │  order_numbers (e.g. "ORD-12345")
    │
    ├─ CALL lookup_order()           ← reads orders.json, zero LLM cost
    │       │  order_context (full JSON record or "not found" message)
    │
    ├─ GENERATE classify_ticket()    ← LLM, grounded with order data
    ├─ GENERATE extract_ticket_details()
    ├─ GENERATE detect_urgency()     → urgency_score 0–10
    │
    ├─ EVALUATE urgency_score
    │     WHEN > 8 → GENERATE escalation_alert()   → COMMIT escalated
    │     OTHERWISE
    │         ├─ GENERATE draft_response()          ← uses order_context
    │         ├─ GENERATE check_response_quality()  → quality_score 0–10
    │         └─ EVALUATE quality_score
    │               WHEN < 6 → GENERATE improve_response() → COMMIT drafted_revised
    │               OTHERWISE                              → COMMIT drafted
    │
    └─ EXCEPTION → GENERATE fallback_response()    → COMMIT fallback
```

## Python tools (`tools.py`)

### `extract_order_numbers(text)`
Regex-based extraction — no LLM. Recognises:
- `ORD-12345` — explicit prefixed ID
- `ORDER-12345` — alternative prefix
- `#12345` — hash notation
- `order #12345` / `order number 12345` — natural language

Returns comma-separated normalised IDs (`ORD-12345, ORD-67890`) or empty string.

### `lookup_order(order_numbers)`
Reads `orders.json` and returns the full order record(s) as JSON, or a descriptive message when:
- `order_numbers` is empty → `"No order number found in this ticket."`
- ID not in database → asks LLM to request confirmation from customer

## Sample orders (`orders.json`)

| Order ID | Customer | Status | Scenario |
|---|---|---|---|
| `ORD-12345` | Alice Johnson | delivered | **Duplicate charge** — two charges CHG-8801 and CHG-8802 |
| `ORD-67890` | Bob Martinez | shipped | **Stuck in transit** — no carrier scan for 7 days |
| `ORD-11111` | Carol Kim | delivered | **Wrong item** — fulfillment mispick confirmed |
| `ORD-22222` | David Okonkwo | delivered | **Refund requested** — within 30-day window, eligible |
| `ORD-33333` | Eva Lindqvist | cancelled | **Cancelled** — full refund already issued |
| `ORD-44444` | Frank Torres | processing | **New order** — in pick queue, ships within 24h |
| `ORD-55555` | Grace Nguyen | delivered | **API errors** — engineering ticket ENG-4421 open |
| `ORD-88888` | Henry Park | delivered | **No issues** — clean delivered order |

## Categories and routing

| Category | Team | SLA | Priority |
|---|---|---|---|
| `billing` | Finance | 4h | High |
| `account` | Auth | 2h | High |
| `shipping` | Fulfillment | 6h | High |
| `technical` | Engineering | 8h | Medium |
| `product` | Product | 24h | Medium |
| `complaint` | Customer Success | 1h | Urgent |
| `general` | First-line | 48h | Low |

## Output status

| Status | Meaning |
|---|---|
| `escalated` | Urgency > 8 — escalation alert committed |
| `drafted` | Quality check passed (score ≥ 6) |
| `drafted_revised` | Quality was low — one revision applied |
| `fallback` | GenerationError — safe fallback response used |
