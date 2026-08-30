# Recipe 33: Interview Simulator

Two-persona structured Q&A with scoring and evaluation. Loads structured role and candidate
data from a catalog, generates interview questions grounded in real competencies, simulates
candidate answers with realistic strengths and gaps, scores each answer on four dimensions,
and produces a verdict and narrative evaluation report.

## What's in this recipe

| File | Purpose |
|---|---|
| `interview_sim.spl` | Main SPL workflow |
| `tools.py` | Python tools: `load_role`, `load_candidate`, `list_roles`, `extract_question`, `aggregate_scores`, `compile_transcript` |
| `data/roles.json` | 3 roles × 2-3 focus areas, with sample questions and answer signals |
| `data/candidates.json` | 5 candidate profiles with backgrounds, strengths, gaps, and communication style |

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `role_key` | TEXT | `''` | Role ID from catalog (`senior_swe`, `data_scientist`, `devops_engineer`) |
| `candidate_id` | TEXT | `''` | Candidate ID from catalog (e.g. `alice_senior_swe`, `carol_data_scientist`) |
| `role` | TEXT | `''` | Freeform role title (used when role_key is blank) |
| `focus` | TEXT | `''` | Focus area key or description (e.g. `system_design`, `machine_learning`) |
| `difficulty` | TEXT | `medium` | `easy`, `medium`, or `hard` |
| `num_questions` | INT | `3` | Number of interview questions |
| `experience` | TEXT | `5 years` | Fallback when no candidate profile is loaded |

Pass `role_key` + `focus` + `candidate_id` for catalog-grounded simulation (recommended).
Pass `role` + `focus` alone for a freeform run with no catalog data.

## Usage

Always pass `--tools tools.py`:

```bash
# Senior SWE — system design — experienced candidate (hard)
spl-go run cookbook/33_interview_sim/interview_sim.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/33_interview_sim/tools.py \
    role_key=senior_swe focus=system_design candidate_id=alice_senior_swe difficulty=hard \
    2>&1 | tee cookbook/out/33_interview_sim-$(date +%Y%m%d_%H%M%S).md

# Senior SWE — algorithms — junior candidate (interesting gap)
spl-go run cookbook/33_interview_sim/interview_sim.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/33_interview_sim/tools.py \
    role_key=senior_swe focus=algorithms candidate_id=eve_junior_swe

# Data scientist — statistics — PhD candidate
spl-go run cookbook/33_interview_sim/interview_sim.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/33_interview_sim/tools.py \
    role_key=data_scientist focus=statistics candidate_id=carol_data_scientist difficulty=hard

# DevOps — Kubernetes — mid-level candidate
spl-go run cookbook/33_interview_sim/interview_sim.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/33_interview_sim/tools.py \
    role_key=devops_engineer focus=kubernetes candidate_id=dave_devops

# Freeform — no catalog needed
spl-go run cookbook/33_interview_sim/interview_sim.spl \
    --adapter ollama --model gemma3 \
    --tools cookbook/33_interview_sim/tools.py \
    role="ML Engineer" focus="deep learning" difficulty=medium experience="3 years"

# Discover available roles / candidates
spl-go run ... role_key=list
spl-go run ... candidate_id=list
```

## Workflow steps

```
role_key + focus  +  candidate_id
    │
    ├─ CALL load_role()              ← competencies, sample Qs, answer signals — zero LLM cost
    ├─ CALL load_candidate()         ← background, strengths, gaps, style — zero LLM cost
    │
    ├─ GENERATE generate_question_set()         ← LLM, grounded by role context
    │
    ├─ CALL extract_question(json, '1')         ← deterministic JSON parse — zero LLM cost
    ├─ CALL extract_question(json, '2')         ← (replaces 3 × GENERATE ask_question)
    ├─ CALL extract_question(json, '3')         ←
    │
    ├─ GENERATE answer_question() × 3          ← LLM, grounded by candidate profile
    ├─ GENERATE score_answer()     × 3          ← LLM, uses rubric + role context
    │
    ├─ CALL aggregate_scores()      ← averages + verdict — zero LLM cost
    ├─ CALL compile_transcript()    ← formatted text — zero LLM cost
    │
    └─ GENERATE overall_evaluation()            ← LLM narrative with deterministic scores

    EXCEPTION GenerationError → COMMIT transcript as partial
```

## Python tools (`tools.py`)

### `load_role(role_key, focus_area)`
Returns competencies, 5 sample questions, and strong/weak answer signals for the focus area.
Grounds `generate_question_set` and `score_answer` with role-appropriate expectations.

### `load_candidate(candidate_id)`
Returns background, strong areas, weak areas, communication style, and typical answer gaps.
Grounds `answer_question` so simulated answers are realistic — confident on strengths, honest on gaps.

### `list_roles()`
Lists all role IDs, titles, and available focus areas.

### `extract_question(questions_json, n)`
Deterministically pulls question N from the LLM-generated JSON array.
Replaces 3 × `GENERATE ask_question()` — zero tokens.

### `aggregate_scores(score1, score2, score3)`
Parses the three score JSONs and computes per-dimension averages, overall total, and a verdict:

| Overall % | Verdict |
|---|---|
| ≥ 85% | strong hire |
| ≥ 70% | hire |
| ≥ 55% | lean hire |
| < 55% | no hire |

### `compile_transcript(q1, a1, score1, q2, a2, score2, q3, a3, score3, role, focus)`
Formats the full Q&A as a readable transcript with inline score summaries. Zero LLM cost.

## Role catalog (`data/roles.json`)

| `role_key` | Title | Focus areas |
|---|---|---|
| `senior_swe` | Senior Software Engineer | `system_design`, `algorithms`, `behavioral` |
| `data_scientist` | Data Scientist | `machine_learning`, `statistics`, `python_and_data_engineering` |
| `devops_engineer` | DevOps / Platform Engineer | `kubernetes`, `cicd`, `cloud_infrastructure` |

## Candidate catalog (`data/candidates.json`)

| `candidate_id` | Name | Role | Exp | Notable traits |
|---|---|---|---|---|
| `alice_senior_swe` | Alice Chen | senior_swe | 6y | Strong distributed systems; weak formal algorithms |
| `bob_mid_swe` | Bob Ramirez | senior_swe | 3y | Enthusiastic; shallow on system design depth |
| `carol_data_scientist` | Carol Osei | data_scientist | 5y | PhD, rigorous stats; weak on deployment/MLOps |
| `dave_devops` | Dave Kim | devops_engineer | 4y | Strong IaC/CI; weak on multi-region and security depth |
| `eve_junior_swe` | Eve Johansson | senior_swe | 2y | Clean code, honest about gaps; limited scale experience |

## Evaluation rubric (per question)

| Dimension | Scale | Description |
|---|---|---|
| `accuracy` | 0–10 | Technically correct and free of errors |
| `depth` | 0–10 | Goes beyond surface level |
| `communication` | 0–10 | Clear, structured, easy to follow |
| `experience` | 0–10 | References concrete real-world scenarios |
| `total` | 0–40 | Sum of all four |

## Output status

| Status | Meaning |
|---|---|
| `complete` | Full transcript + aggregate scores + overall evaluation |
| `partial` | GenerationError — transcript committed without final evaluation |
