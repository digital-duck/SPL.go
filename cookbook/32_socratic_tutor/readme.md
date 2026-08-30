# Recipe 32: Socratic Tutor

Guides a student to discover answers through questions rather than direct instruction.
Loads structured topic context from a catalog, adapts question complexity to the student's
level, simulates a 3-turn dialogue, assesses understanding, and formats the result deterministically.

## What's in this recipe

| File | Purpose |
|---|---|
| `socratic_tutor.spl` | Main SPL workflow |
| `tools.py` | Python tools: `load_topic`, `list_topics`, `get_level_guidance`, `compile_dialogue` |
| `topics/science.json` | 5 science topics (optics, biology, physics, earth science, evolution) |
| `topics/math.json` | 5 math topics (proof, probability, calculus, set theory, geometry) |
| `topics/programming.json` | 5 CS topics (recursion, Big O, pointers, async, OOP) |

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `topic_id` | TEXT | `''` | Topic ID from the catalog (e.g. `sky_blue`, `recursion`) |
| `subject` | TEXT | `''` | Subject file to look up: `science`, `math`, `programming` |
| `topic` | TEXT | `''` | Freeform topic description (used when no catalog entry exists) |
| `student_level` | TEXT | `high school` | `elementary`, `middle school`, `high school`, `undergraduate`, `graduate`, `expert` |
| `max_questions` | INT | `5` | Maximum questions in the dialogue |

Pass `topic_id` + `subject` to use the catalog (recommended — gives richer context).
Pass `topic` alone for any freeform question not in the catalog.

## Usage

Always pass `--tools tools.py`:

```bash
# Science — sky blue (middle school)
spl-go run cookbook/32_socratic_tutor/socratic_tutor.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/32_socratic_tutor/tools.py \
    topic_id=sky_blue subject=science student_level="middle school" \
    2>&1 | tee cookbook/out/32_socratic-$(date +%Y%m%d_%H%M%S).md

# Math — Monty Hall problem (undergraduate)
spl-go run cookbook/32_socratic_tutor/socratic_tutor.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/32_socratic_tutor/tools.py \
    topic_id=monty_hall subject=math student_level=undergraduate

# Math — sqrt(2) irrational proof (high school)
spl-go run cookbook/32_socratic_tutor/socratic_tutor.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/32_socratic_tutor/tools.py \
    topic_id=sqrt2_irrational subject=math student_level="high school"

# Programming — recursion (high school)
spl-go run cookbook/32_socratic_tutor/socratic_tutor.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/32_socratic_tutor/tools.py \
    topic_id=recursion subject=programming student_level="high school"

# Freeform topic (no catalog entry needed)
spl-go run cookbook/32_socratic_tutor/socratic_tutor.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/32_socratic_tutor/tools.py \
    topic="Why is the speed of light constant?" student_level=undergraduate
```

## Workflow steps

```
topic_id + subject  (or freeform topic)
    │
    ├─ CALL load_topic()           ← prerequisites, objectives, misconceptions, Socratic path
    ├─ CALL get_level_guidance()   ← vocabulary + scaffolding hints for the student level
    │
    ├─ GENERATE opening_question()                  ← LLM, grounded by topic context
    ├─ GENERATE simulate_student_response() [1]     ← LLM, typical student at given level
    ├─ GENERATE followup_question()                 ← LLM, builds on student's actual words
    ├─ GENERATE simulate_student_response() [2]     ← LLM
    ├─ GENERATE assess_understanding()              ← LLM → score 0–10
    │
    ├─ EVALUATE understanding_score
    │     WHEN > 7 → GENERATE consolidation_question()  ← cement understanding
    │     OTHERWISE → GENERATE hint_question()           ← unblock with scaffolding
    │
    ├─ GENERATE simulate_student_response() [3]     ← LLM
    └─ CALL compile_dialogue()     ← deterministic formatting, zero LLM cost

    COMMIT with status='complete', understanding_score
```

## Python tools (`tools.py`)

### `load_topic(topic_id, subject)`
Returns a structured text block with prerequisites, learning objectives, common misconceptions,
key concepts, and a suggested Socratic question path. This context grounds every GENERATE call
so the LLM never hallucinates misconceptions or uses vocabulary above the student's level.

Pass `topic_id=""` to list all topics for the subject.

### `list_topics(subject)`
Lists all topic IDs and titles for `science`, `math`, or `programming`.

### `get_level_guidance(student_level)`
Returns vocabulary rules, question complexity guidance, and scaffolding strategy for levels:
`elementary`, `middle school`, `high school`, `undergraduate`, `graduate`, `expert`.

### `compile_dialogue(q1, s1, q2, s2, q3, s3, topic, understanding_score)`
Formats the three-turn Socratic dialogue as a clean text transcript with a header, dividers,
and an understanding score label. Deterministic — no LLM cost.

## Topic catalog

### Science (`subject=science`)
| topic_id | Title | Level |
|---|---|---|
| `sky_blue` | Why does the sky appear blue? | middle school, high school |
| `photosynthesis` | How do plants make food from sunlight? | middle school, high school |
| `gravity_freefall` | Why do objects fall at the same rate regardless of mass? | high school, undergraduate |
| `climate_change` | Why does carbon dioxide cause global warming? | high school, undergraduate |
| `natural_selection` | How does natural selection drive evolution? | high school, undergraduate |

### Math (`subject=math`)
| topic_id | Title | Level |
|---|---|---|
| `sqrt2_irrational` | Prove that sqrt(2) is irrational | high school, undergraduate |
| `monty_hall` | Why should you always switch doors in the Monty Hall problem? | high school, undergraduate |
| `limits` | What is a limit in calculus and why does it matter? | high school, undergraduate |
| `infinity_sizes` | Are some infinities bigger than others? | undergraduate, expert |
| `pythagorean_theorem` | Why does a² + b² = c²? | middle school, high school |

### Programming (`subject=programming`)
| topic_id | Title | Level |
|---|---|---|
| `recursion` | What is recursion and how does it work? | high school, undergraduate |
| `big_o` | What is Big O notation and why does it matter? | high school, undergraduate |
| `pointers` | What is a pointer and why does it matter? | undergraduate |
| `async_programming` | What is asynchronous programming and when do you need it? | undergraduate, expert |
| `oop_principles` | What is object-oriented programming and why was it invented? | high school, undergraduate |

## Output

The dialogue is formatted as:

```
SOCRATIC DIALOGUE
Topic: Why does the sky appear blue?
────────────────────────────────────────────────────────────

TUTOR:   What do you already know about what sunlight is made of?

STUDENT: I think sunlight is just... white? Like it's just bright light.

────────────────────────────────────────────────────────────

TUTOR:   Interesting — have you ever seen what happens when sunlight passes through a prism?

...

Understanding score: 7.5/10 — good progress with room to grow
```

## Notes

- The tutor persona never gives direct answers — all GENERATE steps are constrained by the `socratic_persona()` function.
- `compile_dialogue` is deterministic (CALL, not GENERATE) — formatting never costs tokens.
- Understanding score > 7 → consolidation question to cement knowledge; ≤ 7 → hint question to unblock the student.
