# Recipe 29: Meeting Notes → Action Items

Transcript in, structured output. Normalizes the transcript, extracts action items with owners
and priorities, normalises relative due dates to ISO-8601, validates for unowned high-priority
items, and formats as JSON, Markdown, or email.

## What's in this recipe

| File | Purpose |
|---|---|
| `meeting_actions.spl` | Main SPL workflow |
| `tools.py` | Python tools: `load_transcript`, `extract_speakers`, `normalize_dates`, `validate_ownership` |
| `transcripts/sprint_planning.txt` | Full sprint planning meeting (5 speakers, ~15 action items) |
| `transcripts/design_review.txt` | API architecture review with open questions and unowned tasks |
| `transcripts/standup.txt` | Short daily standup with blockers |

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `filename` | TEXT | `''` | Transcript filename inside `transcripts/` (e.g. `sprint_planning.txt`) |
| `transcript` | TEXT | `''` | Inline transcript text (alternative to `filename`) |
| `output_format` | TEXT | `json` | Output format: `json`, `markdown`, or `email` |

Pass either `filename` or `transcript` — if both are given, `filename` takes precedence.

## Usage

Always pass `--tools tools.py` so the deterministic tools are available:

```bash
# Sprint planning → markdown (good for copy-pasting into Confluence)
spl-go run cookbook/29_meeting_actions/meeting_actions.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/29_meeting_actions/tools.py \
    filename=sprint_planning.txt \
    output_format=markdown \
    2>&1 | tee cookbook/out/29_meeting_actions-$(date +%Y%m%d_%H%M%S).md

# Design review → JSON (inspect structured output)
spl-go run cookbook/29_meeting_actions/meeting_actions.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/29_meeting_actions/tools.py \
    filename=design_review.txt \
    output_format=json

# Daily standup → email format
spl-go run cookbook/29_meeting_actions/meeting_actions.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/29_meeting_actions/tools.py \
    filename=standup.txt \
    output_format=email

# Inline transcript (no file needed)
spl-go run cookbook/29_meeting_actions/meeting_actions.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/29_meeting_actions/tools.py \
    transcript="Alice: fix the login bug by Friday. Bob: I will handle it." \
    output_format=markdown
```

## Workflow steps

```
filename / transcript
    │
    ├─ CALL load_transcript()        ← reads file from transcripts/, zero LLM cost
    ├─ CALL extract_speakers()       ← regex, zero LLM cost (x2: file + inline)
    │
    ├─ GENERATE normalize_transcript()  ← LLM: merge sources, clean formatting
    ├─ GENERATE extract_actions()       ← LLM: produce structured JSON per schema
    │
    ├─ CALL normalize_dates()        ← convert "Friday" → "2026-03-27", zero LLM cost
    ├─ CALL validate_ownership()     ← flag high-priority items with no owner, zero LLM cost
    │
    └─ EVALUATE output_format
          markdown → GENERATE format_as_markdown()  → COMMIT complete/markdown
          email    → GENERATE format_as_email()     → COMMIT complete/email
          json     → SET @output = @structured_json  → COMMIT complete/json

    EXCEPTION ContextLengthExceeded
          → GENERATE summarize_transcript() → GENERATE extract_actions() → COMMIT complete_chunked
```

## Python tools (`tools.py`)

### `load_transcript(filename)`
Loads a `.txt` file from the `transcripts/` folder by name.
Returns full text, or an error message listing available files if not found.

### `extract_speakers(text)`
Regex-extracts unique speaker names from `"Speaker: ..."` lines and the `Attendees:` header.
Returns a comma-separated list, e.g. `"Alice, Bob, Carol, Dave, Eve"`.
Passed to the LLM so owners in action items match exact speaker names (no hallucinated names).

### `normalize_dates(json_str)`
Walks every `due_date` field and converts relative phrases to ISO-8601 using the current date:

| Phrase | Resolved to |
|---|---|
| `today`, `end of day`, `eod` | today |
| `tomorrow` | today + 1 day |
| `Friday`, `this Friday` | next Friday |
| `next Wednesday` | Wednesday at least 7 days away |
| `this week`, `end of week`, `eow` | next Friday |
| `end of sprint`, `end of this sprint` | Friday + 1 week |
| `next sprint` | today + 2 weeks |
| `end of month` | last day of current month |

### `validate_ownership(json_str)`
Checks every action item with `priority = "high"`. If `owner` is empty, TBD, or unassigned,
reports it as a warning. The LLM uses this report when drafting the formatted output.

## Output fields (JSON format)

| Field | Description |
|---|---|
| `meeting_title` | Title extracted from the transcript header |
| `meeting_date` | Date of the meeting |
| `attendees` | List of participant names |
| `meeting_summary` | One-paragraph summary |
| `decisions` | List of explicit decisions made |
| `action_items` | List of tasks with `task`, `owner`, `due_date` (ISO-8601), `priority`, `context` |
| `open_questions` | Unresolved questions flagged during the meeting |
| `next_meeting` | Next meeting date/time if mentioned |

## Sample transcripts

| File | Meeting type | Speakers | Key scenarios |
|---|---|---|---|
| `sprint_planning.txt` | Sprint planning | Alice, Bob, Carol, Dave, Eve | Story assignments, tech debt, unowned infra tasks, open question on mobile timeline |
| `design_review.txt` | Architecture review | Priya, Sam, Jordan, Maya, Felix | Kafka cluster decision, schema versioning, unowned DLQ strategy, security spec |
| `standup.txt` | Daily standup | Alice, Bob, Carol, Dave | Blockers, quick unblocks, estimate request |

## Output status

| Status | Meaning |
|---|---|
| `complete` | All steps succeeded |
| `complete_chunked` | Transcript was too long — summarised before extraction |
