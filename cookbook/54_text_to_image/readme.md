# Recipe 52: Text to Image

**Category:** multimodal / generative  
**SPL version:** 3.0  
**Type:** TEXT → IMAGE  
**LLM required:** Yes — gemma4 (prompt enhancement, Ollama) + DALL-E 3 (image generation, OpenAI)  
**Demonstrates:** `IMAGE` as a first-class SPL output type, two-stage generation pipeline, `EVALUATE @enhance`

---

## What it does

Generates a high-quality image from a text prompt using DALL-E 3, with an
optional prompt enhancement step via local gemma4.

```
[text prompt]
      │
      ▼  gemma4 via Ollama (optional — @enhance = TRUE)
[vivid, detailed enhanced prompt]
      │
      ▼  DALL-E 3 via OpenAI
[image file saved to @output_dir]
```

The prompt enhancement step uses a local model to add lighting, camera angle,
mood, and style-specific keywords — improving output quality without requiring
the user to learn prompt engineering.

---

## Files

| File | Role |
|------|------|
| `text_to_image.spl` | SPL logical view — 1 prompt function, 1 workflow |
| `run.py` | Physical runner — gemma4 enhancement + DALL-E 3 API call |
| `outputs/` | Generated images written here |

---

## Prerequisites

```bash
export OPENAI_API_KEY=sk-...   # must point to api.openai.com (not OpenRouter)

# For prompt enhancement (optional but recommended)
ollama serve
ollama pull gemma4:e4b
```

> **Note:** DALL-E 3 is only available via the official OpenAI API (`api.openai.com`).
> OpenRouter does not proxy DALL-E 3.

---

## Running

```bash
export OPENAI_API_KEY=sk-...
export OPENAI_BASE_URL="https://api.openai.com/v1"

# Basic prompt
python cookbook/54_text_to_image/run.py \
    --prompt "A duck writing code at sunrise"

[text_to_image] → DALL-E dall-e-3 (1024x1024, quality=standard) ...
[text_to_image] DALL-E revised prompt: An engaging scene featuring a duck. The duck is sitting at a computer desk, its ...
[text_to_image] ✓ saved generated_1776561500.png (1335 KB, 16561 ms)

── Output ───────────────────────────────────────────────────────────
Image saved: cookbook/54_text_to_image/outputs/generated_1776561500.png


# Landscape HD with vivid style
python cookbook/54_text_to_image/run.py \
    --prompt "NYC skyline at night" \
    --aspect landscape --quality hd --dalle-style vivid

# With gemma4 prompt enhancement
python cookbook/54_text_to_image/run.py \
    --prompt "A fox in a forest" \
    --enhance --style "oil painting, impressionist"

# No enhancement (send raw prompt directly to DALL-E 3)
python cookbook/54_text_to_image/run.py \
    --prompt "A serene mountain lake" \
    --enhance false

# Custom output directory
python cookbook/54_text_to_image/run.py \
    --prompt "A futuristic city" \
    --output-dir /tmp/images
```

---

## Parameters

### SPL workflow params

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@prompt` | TEXT | `A serene mountain lake at golden hour...` | Base text prompt |
| `@style` | TEXT | `photorealistic` | Visual style applied during enhancement |
| `@aspect` | TEXT | `landscape` | Aspect ratio: `landscape`, `portrait`, `square` |
| `@quality` | TEXT | `hd` | DALL-E 3 quality: `standard` or `hd` |
| `@dalle_style` | TEXT | `natural` | `natural` (realistic) \| `vivid` (dramatic) |
| `@enhance` | BOOL | `TRUE` | Run gemma4 prompt enhancement before generation |
| `@model` | TEXT | `dall-e-3` | Image generation model |
| `@output_dir` | TEXT | `cookbook/54_text_to_image/outputs` | Directory to write generated image |
| `@llm_model` | TEXT | `gemma4:e4b` | Local LLM for prompt enhancement |
| `@log_dir` | TEXT | `cookbook/54_text_to_image/logs-spl` | Directory for log output |

### CLI flags (run.py)

| Flag | Default | Description |
|------|---------|-------------|
| `--prompt` | *(required)* | Text description of the image |
| `--style` | `photorealistic` | Style hint for prompt enhancement |
| `--aspect` | `square` | `square` \| `landscape` \| `portrait` |
| `--quality` | `standard` | `standard` \| `hd` (costs ~2× more) |
| `--dalle-style` | `natural` | `natural` (realistic) \| `vivid` (dramatic) |
| `--enhance` | off | Run prompt through Gemma4 before DALL-E |
| `--model` | `dall-e-3` | `dall-e-3` \| `dall-e-2` |
| `--llm-model` | `gemma4:e4b` | Ollama model for enhancement |
| `--output-dir` | `cookbook/54_text_to_image/outputs` | Directory to write generated image |

---

## Output

`@image_path IMAGE` — file path of the generated image in `@output_dir`.

Filename pattern: `generated_<timestamp>.png`

---

## Composability

```sql
-- Generate an image, then caption it with gemma4 vision
CALL text_to_image(@prompt, @style, ...) INTO @image
CALL image_caption(@image, 'What does this image show?', 'caption', @model, 512) INTO @caption

-- Generate then restyle it
CALL text_to_image(@prompt, ...) INTO @image
CALL image_restyle(@image, 'watercolor painting', ...) INTO @restyled
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | `OPENAI_API_KEY` missing or DALL-E quota exceeded | Returns error string with `status = 'failed'` |
