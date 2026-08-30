# Recipe 39: Vertex AI Quickstart

Fan out the same prompt across three Gemini model tiers (Pro / Flash / Lite) on Google Cloud Vertex AI and compare quality, speed, and cost. Useful for choosing the right Gemini tier for a given workload in a GCP environment.

## Prerequisites

**1. Install the SDK**
```bash
pip install google-genai
```

**2. Authenticate** (choose one)

```bash
# Option A — local development (uses your gcloud identity)
gcloud auth application-default login

# Option B — service account key
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Option C — Workload Identity / attached SA (GKE, Cloud Run, GCE)
# No config needed — the metadata server provides credentials automatically.
```

**3. Set your project**
```bash
export GOOGLE_CLOUD_PROJECT=my-gcp-project-id
# Region defaults to us-central1 — override with:
export GOOGLE_CLOUD_LOCATION=europe-west4
```

**4. Enable the Vertex AI API**
```bash
gcloud services enable aiplatform.googleapis.com --project=$GOOGLE_CLOUD_PROJECT
```

## Usage

```bash
# Default — Gemini 2.5 Pro vs Flash vs 2.0 Flash Lite
spl-go run cookbook/39_vertex_quickstart/vertex_quickstart.spl --adapter vertex \
    prompt="Explain the CAP theorem in two sentences."

# Custom models
spl-go run cookbook/39_vertex_quickstart/vertex_quickstart.spl --adapter vertex \
    prompt="What are the trade-offs between REST and GraphQL?" \
    model_pro="gemini-2.5-pro" \
    model_flash="gemini-2.5-flash" \
    model_lite="gemini-2.0-flash-lite"

# Different region
GOOGLE_CLOUD_LOCATION=europe-west4 \
spl-go run cookbook/39_vertex_quickstart/vertex_quickstart.spl --adapter vertex \
    prompt="Summarise the benefits of serverless architectures."
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `prompt` | TEXT | `Explain the CAP theorem in two sentences.` | The question sent to all three models |
| `model_pro` | TEXT | `gemini-2.5-pro` | Pro tier — highest capability |
| `model_flash` | TEXT | `gemini-2.5-flash` | Flash tier — balanced quality/cost |
| `model_lite` | TEXT | `gemini-2.0-flash-lite` | Lite tier — fastest, lowest cost |

## How it works

Three CTEs fan out the prompt to each Gemini tier in parallel on Vertex AI. A judge step (also on Vertex) evaluates accuracy, conciseness, and production fit.

```
prompt
  ├─► gemini-2.5-pro        → @answer_pro
  ├─► gemini-2.5-flash      → @answer_flash    ─► compare_tiers() → @comparison
  └─► gemini-2.0-flash-lite → @answer_lite
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | All three tiers responded, comparison committed |
| `error` | One or more models failed — check project, IAM, and API enablement |

## Available Vertex AI Models

| Model | Tier | Strengths |
|---|---|---|
| `gemini-2.5-pro` | Pro | Complex reasoning, long context, highest quality |
| `gemini-2.5-flash` | Flash | Strong quality at lower cost, recommended default |
| `gemini-2.0-flash` | Flash | Well-tested, balanced |
| `gemini-2.0-flash-lite` | Lite | Lowest latency and cost for simple tasks |
| `gemini-1.5-pro` | Pro | Proven long-context model |
| `gemini-1.5-flash` | Flash | Production-stable fast tier |

## Approximate Vertex AI pricing (per 1M tokens)

| Model | Input | Output |
|---|---|---|
| Gemini 2.5 Pro | $1.25 | $10.00 |
| Gemini 2.5 Flash | $0.15 | $0.60 |
| Gemini 2.0 Flash | $0.10 | $0.40 |
| Gemini 2.0 Flash Lite | $0.075 | $0.30 |
| Gemini 1.5 Pro | $1.25 | $5.00 |
| Gemini 1.5 Flash | $0.075 | $0.30 |

> Verify current rates at https://cloud.google.com/vertex-ai/generative-ai/pricing

## Required IAM permissions

Your service account or user identity needs:
```
roles/aiplatform.user
```
Or the fine-grained permission: `aiplatform.endpoints.predict`

## Difference from Recipe 03 (Google Adapter)

| | `google` adapter (Recipe 03) | `vertex` adapter (Recipe 39) |
|---|---|---|
| Auth | `GOOGLE_API_KEY` (API key) | ADC / service account / Workload Identity |
| Backend | Google AI Studio | Google Cloud Vertex AI |
| Billing | Google AI quota | GCP project billing |
| VPC / Private networking | No | Yes (via Private Service Connect) |
| Audit logs | No | Cloud Audit Logs |
| Best for | Prototyping, personal projects | Enterprise, production GCP workloads |
