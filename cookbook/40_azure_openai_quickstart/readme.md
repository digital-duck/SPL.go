# Recipe 40: Azure OpenAI Quickstart

Fan out the same prompt across three Azure OpenAI deployments (GPT-4o / GPT-4o mini / GPT-3.5 Turbo by default) and compare responses. Useful for validating model selection and cost trade-offs within your own Azure subscription.

## Prerequisites

**1. Install the SDK**
```bash
pip install openai
```

**2. Create an Azure OpenAI resource**

In the [Azure portal](https://portal.azure.com):
- Create an **Azure OpenAI** resource
- Go to **Azure OpenAI Studio → Deployments** and create deployments for the models you want (e.g. `gpt-4o`, `gpt-4o-mini`, `gpt-35-turbo`)

> Deployment names are what you pass as `model=` in SPL — they may differ from the underlying model name.

**3. Configure credentials**

```bash
export AZURE_OPENAI_ENDPOINT=https://<your-resource-name>.openai.azure.com/
export AZURE_OPENAI_API_KEY=<your-key-from-azure-portal>
# API version defaults to 2025-01-01-preview — override if needed:
export AZURE_OPENAI_API_VERSION=2024-12-01-preview
```

## Usage

```bash
# Default — gpt-4o vs gpt-4o-mini vs gpt-35-turbo deployments
spl-go run cookbook/40_azure_openai_quickstart/azure_openai_quickstart.spl --adapter azure_openai \
    prompt="Explain the CAP theorem in two sentences."

# Custom deployment names (must match what you created in Azure)
spl-go run cookbook/40_azure_openai_quickstart/azure_openai_quickstart.spl --adapter azure_openai \
    prompt="What are the trade-offs between microservices and monoliths?" \
    deployment_1="gpt-4o" \
    deployment_2="gpt-4o-mini" \
    deployment_3="gpt-35-turbo"

# Single deployment (set all three to the same name)
spl-go run cookbook/40_azure_openai_quickstart/azure_openai_quickstart.spl --adapter azure_openai \
    prompt="Summarise the SOLID principles." \
    deployment_1="gpt-4o" \
    deployment_2="gpt-4o" \
    deployment_3="gpt-4o"
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `prompt` | TEXT | `Explain the CAP theorem in two sentences.` | The question sent to all deployments |
| `deployment_1` | TEXT | `gpt-4o` | First deployment name |
| `deployment_2` | TEXT | `gpt-4o-mini` | Second deployment name |
| `deployment_3` | TEXT | `gpt-35-turbo` | Third deployment name |

## How it works

Three CTEs fan out the prompt to each Azure OpenAI deployment in parallel. A judge step (also on Azure) evaluates accuracy, clarity, and cost-effectiveness.

```
prompt
  ├─► gpt-4o         → @answer_1
  ├─► gpt-4o-mini    → @answer_2   ─► compare_deployments() → @comparison
  └─► gpt-35-turbo   → @answer_3
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | All three deployments responded, comparison committed |
| `error` | One or more deployments failed — check endpoint, API key, or deployment names |

## Common Azure OpenAI Deployment Models

| Model | Deployment name (typical) | Best for |
|---|---|---|
| GPT-4o | `gpt-4o` | High-quality reasoning and coding |
| GPT-4o mini | `gpt-4o-mini` | Cost-efficient, most tasks |
| GPT-4 Turbo | `gpt-4-turbo` | Long context (128K) |
| GPT-3.5 Turbo | `gpt-35-turbo` | Simple tasks, lowest cost |
| o1 | `o1` | Complex multi-step reasoning |
| o1-mini | `o1-mini` | Faster reasoning at lower cost |
| o3-mini | `o3-mini` | Advanced reasoning, efficient |

> Deployment names are set by you in Azure OpenAI Studio — they do not have to match the model names above.

## Approximate Azure OpenAI pricing (per 1M tokens, Pay-as-you-go)

| Model | Input | Output |
|---|---|---|
| GPT-4o | $2.50 | $10.00 |
| GPT-4o mini | $0.15 | $0.60 |
| GPT-4 Turbo | $10.00 | $30.00 |
| GPT-3.5 Turbo | $0.50 | $1.50 |

> Verify current rates at https://azure.microsoft.com/pricing/details/cognitive-services/openai-service/

## Difference from Recipe using `openai` adapter

| | `openai` adapter | `azure_openai` adapter |
|---|---|---|
| Auth | `OPENAI_API_KEY` | `AZURE_OPENAI_API_KEY` + endpoint |
| Endpoint | `api.openai.com` | Your Azure resource endpoint |
| Billing | OpenAI account | Azure subscription |
| Data residency | OpenAI default | Your Azure region |
| Private networking | No | Yes (Private Endpoints) |
| Compliance | OpenAI policies | Azure compliance (HIPAA, SOC2, etc.) |
| Best for | Prototyping | Enterprise, regulated industries |
