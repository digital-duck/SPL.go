# Recipe 20: Ensemble Voting

Generates N independent candidate answers from a pool of models, scores each with a
*different* random model, finds consensus, selects the winner deterministically, and polishes it.

Two versions:

| File | Description |
|---|---|
| `ensemble.spl` | v1 — single model, hardcoded 5 candidates |
| `ensemble_v2.spl` | v2 — multi-model pool, random selection, deterministic winner |

---

## v1 Pattern

```
5 × answer_candidate(question)   [single model]
  └─► 5 × score_candidate        [single model]
        └─► find_consensus
              └─► GENERATE select_winner   ← LLM picks winner
                    └─► polish → final_answer
```

## v2 Pattern

```
@models pool (LIST)
     │
     ├─ pick_random_model()      ──► GENERATE answer_candidate()  USING MODEL @gen_model
     │
     └─ pick_random_model(exclude=@gen_model) ──► GENERATE score_candidate()  USING MODEL @score_model
                                                           │
                                                  GENERATE find_consensus()  USING MODEL @scorer_model
                                                           │
                                                  CALL select_winner()   ← deterministic argmax, zero tokens
                                                           │
                                                  GENERATE polish()  USING MODEL @polish_model
```

Key improvements over v1:
- **Two selection modes** — `random_selection=true` (default) for diversity; `false` for reproducible positional rotation
- **Independent scoring** — scorer is always a different model from the generator (no self-grading)
- **Deterministic winner** — `select_winner` is a Python `CALL` (argmax), not an LLM guess
- **Dedicated polish model** — best model reserved for the final pass
- **Full audit trail** — every step written to `@log_dir`

---

## Parameters

### v1 — `ensemble.spl`

| Parameter | Type | Default | Description |
|---|---|---|---|
| `question` | TEXT | *(required)* | The question to answer |

### v2 — `ensemble_v2.spl`

| Parameter | Type | Default | Description |
|---|---|---|---|
| `question` | TEXT | *(required)* | The question to answer |
| `models` | LIST | `['llama3.2','qwen2.5','gemma3','mistral','deepseek-r1']` | Pool of models to draw from |
| `n_candidates` | INT | `5` | Number of candidate answers to generate |
| `random_selection` | BOOL | `TRUE` | `TRUE` = random draw each iteration; `FALSE` = rotate positionally through list |
| `consensus_model` | TEXT | `'qwen2.5'` | Model used for consensus step (separate from generators) |
| `polish_model` | TEXT | `'deepseek-r1'` | Model used for final polish pass |
| `log_dir` | TEXT | `'cookbook/20_ensemble_voting/logs'` | Directory for per-step log files |

---

## Usage

### v1

```bash
spl-go run cookbook/20_ensemble_voting/ensemble.spl --adapter ollama \
    question="What causes inflation?"
```

### v2

```bash
# Minimal — uses all defaults
spl-go run cookbook/20_ensemble_voting/ensemble_v2.spl \
    --adapter ollama \
    --tools cookbook/20_ensemble_voting/tools.py \
    question="What causes inflation?"

# Custom pool, more candidates, random mode (default)
spl-go run cookbook/20_ensemble_voting/ensemble_v2.spl \
    --adapter ollama \
    --tools cookbook/20_ensemble_voting/tools.py \
    question="Is Rust faster than C++?" \
    models="llama3.2,qwen2.5,gemma3,mistral" \
    n_candidates=7 \
    polish_model="deepseek-r1"

# Positional mode — each model gets exactly one turn, fully reproducible
spl-go run cookbook/20_ensemble_voting/ensemble_v2.spl \
    --adapter ollama \
    --tools cookbook/20_ensemble_voting/tools.py \
    question="What is the best database for time-series data?" \
    random_selection=false

# Lean 3-model positional run
spl-go run cookbook/20_ensemble_voting/ensemble_v2.spl \
    --adapter ollama \
    --tools cookbook/20_ensemble_voting/tools.py \
    question="What is the best database for time-series data?" \
    models="gemma3,mistral,deepseek-r1" \
    n_candidates=3 \
    random_selection=false \
    consensus_model="gemma3" \
    polish_model="deepseek-r1"
```

---

## Log files (v2)

All written to `@log_dir` (default: `cookbook/20_ensemble_voting/logs/`):

| File | Contents |
|---|---|
| `candidate_N.md` | Raw answer + which model generated it |
| `score_N.md` | Numeric score + generator/scorer model pair |
| `consensus.md` | Consensus summary across all candidates |
| `winner.md` | Winning candidate (pre-polish) |
| `final_answer.md` | Polished final answer |

---

## Output status

| Status | Meaning |
|---|---|
| `complete` | All N candidates evaluated and polished |
| `partial` | Budget exceeded; winner selected from partial candidates |
