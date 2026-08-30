# Recipe 38: Bedrock Quickstart

Fan out the same prompt across three AWS Bedrock model families and compare responses. Covers Claude (Anthropic) and Nova (Amazon) — useful for model selection and cost/quality trade-off analysis in enterprise environments.

## Prerequisites

**1. Install boto3**
```bash
pip install boto3
```

**2. Configure AWS credentials** (choose one)

```bash
# Option A — environment variables (CI/CD, containers)
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=us-east-1

# Option B — named profile (laptop/workstation)
aws configure --profile my-work-profile
spl-go run ... --adapter bedrock   # reads default profile automatically

# Option C — IAM role (EC2, ECS, Lambda — no config needed)
# BedrockAdapter picks up the instance profile automatically.
```

**3. Enable model access in the AWS Console**
Navigate to **Amazon Bedrock → Model access** and enable the models you want:
- `anthropic.claude-sonnet-4-20250514-v1:0`
- `anthropic.claude-haiku-4-5-20251001-v1:0`
- `amazon.nova-pro-v1:0`

> Model access is per-region. Make sure you enable access in the region set in `AWS_DEFAULT_REGION`.

## Usage

```bash
# Default — Claude Sonnet 4 vs Claude Haiku 4.5 vs Amazon Nova Pro
spl-go run cookbook/38_bedrock_quickstart/bedrock_quickstart.spl --adapter bedrock \
    prompt="Explain the CAP theorem in two sentences."

# Custom models
spl-go run cookbook/38_bedrock_quickstart/bedrock_quickstart.spl --adapter bedrock \
    prompt="What are the trade-offs between SQL and NoSQL?" \
    model_1="anthropic.claude-opus-4-0-20250514-v1:0" \
    model_2="anthropic.claude-3-5-haiku-20241022-v1:0" \
    model_3="amazon.nova-lite-v1:0"

# Override region without changing env
spl-go run cookbook/38_bedrock_quickstart/bedrock_quickstart.spl \
    --adapter bedrock \
    --adapter-region us-west-2 \
    prompt="Summarise the key benefits of serverless architectures."

# Use a named AWS profile
AWS_PROFILE=my-work-profile \
spl-go run cookbook/38_bedrock_quickstart/bedrock_quickstart.spl --adapter bedrock \
    prompt="What is eventual consistency?"
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `prompt` | TEXT | `Explain the CAP theorem in two sentences.` | The question sent to all models |
| `model_1` | TEXT | `anthropic.claude-sonnet-4-20250514-v1:0` | First model (Anthropic Claude Sonnet) |
| `model_2` | TEXT | `anthropic.claude-haiku-4-5-20251001-v1:0` | Second model (Anthropic Claude Haiku) |
| `model_3` | TEXT | `amazon.nova-pro-v1:0` | Third model (Amazon Nova Pro) |

## How it works

Three CTEs fan out the prompt to each Bedrock model in parallel. The results flow into a judge step (also running on Bedrock) that evaluates accuracy, conciseness, and production suitability.

```
prompt
  ├─► Claude Sonnet 4      → @answer_1
  ├─► Claude Haiku 4.5     → @answer_2  ─► compare_models() → @comparison
  └─► Amazon Nova Pro      → @answer_3
```

## Output status

| Status | Meaning |
|---|---|
| `complete` | All three models responded, comparison committed |
| `error` | One or more models failed — check region and model access |

## Available Bedrock Models

| Model ID | Family | Use case |
|---|---|---|
| `anthropic.claude-opus-4-0-20250514-v1:0` | Claude 4 | Complex reasoning, highest quality |
| `anthropic.claude-sonnet-4-20250514-v1:0` | Claude 4 | Balanced quality/cost (default) |
| `anthropic.claude-haiku-4-5-20251001-v1:0` | Claude 4 | Fast, cost-efficient |
| `anthropic.claude-3-5-sonnet-20241022-v2:0` | Claude 3.5 | Strong coding and analysis |
| `anthropic.claude-3-5-haiku-20241022-v1:0` | Claude 3.5 | Fast turnaround |
| `amazon.nova-pro-v1:0` | Nova | Multimodal, AWS-native |
| `amazon.nova-lite-v1:0` | Nova | Lower cost, quick tasks |
| `amazon.nova-micro-v1:0` | Nova | Cheapest option, short tasks |
| `meta.llama3-70b-instruct-v1:0` | Llama 3 | Open-weight, 70B |

Cross-region inference profiles (e.g. `us.anthropic.claude-sonnet-4-20250514-v1:0`) are supported — pass the full profile ARN as `model_1=`.

## Approximate pricing (on-demand, per 1M tokens)

| Model | Input | Output |
|---|---|---|
| Claude Opus 4 | $15.00 | $75.00 |
| Claude Sonnet 4 | $3.00 | $15.00 |
| Claude Haiku 4.5 | $0.25 | $1.25 |
| Amazon Nova Pro | $0.80 | $3.20 |
| Amazon Nova Lite | $0.06 | $0.24 |

> Costs are estimated by the adapter and shown in `spl run` output. Verify current rates at https://aws.amazon.com/bedrock/pricing/

## Notes

**Why not `is_active: true` in the catalog?**
This recipe requires real AWS credentials and model access approval — it cannot run automatically in batch test mode. Set `is_active: true` in `cookbook_catalog.json` once your environment is configured.

**IAM permissions needed**
Your IAM principal needs `bedrock:InvokeModel` (or `bedrock:Converse`) on the target models:
```json
{
  "Effect": "Allow",
  "Action": ["bedrock:InvokeModel", "bedrock:Converse"],
  "Resource": "arn:aws:bedrock:us-east-1::foundation-model/*"
}
```
