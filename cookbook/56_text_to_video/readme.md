# Recipe 59: Text to Video

**Category:** multimodal / generative  
**SPL version:** 3.0  
**Type:** TEXT → VIDEO  
**LLM required:** Yes — gemma4 (prompt enhancement) + video generation model  
**Demonstrates:** VIDEO as a first-class SPL output type, two-stage generation pipeline, `EVALUATE @enhance`

---

## What it does

Generates a short video clip from a natural-language text prompt.

Pipeline:

```
[text prompt]
      │
      ▼  gemma4 (optional — @enhance = TRUE)
[enhanced cinematic prompt]
      │
      ▼  video generation model (Google Veo 2 / RunwayML Gen-3 / Kling AI)
[video file saved to @output_dir]
```

The prompt enhancement step uses a local gemma4 (via Ollama) to add camera
movement, lighting, temporal progression, and cinematic detail before the
prompt is sent to the cloud video model — improving output quality without
requiring the user to learn video-generation prompt engineering.

**NOTE:** `VIDEO` is a first-class SPL 3.0 type (`SPL3Type.VIDEO`).
This is the first recipe in the SPL30 cookbook with `VIDEO` as output.

---

## Files

| File | Role |
|------|------|
| `text_to_video.spl` | SPL logical view — declarative workflow definition |
| `run.py` | Physical runner — gemma4 prompt enhancement + video API call |
| `outputs/` | Generated video files written here |

---

## Prerequisites

### Local (prompt enhancement)
```bash
ollama serve
ollama pull gemma4           # prompt enhancement step
```

### Cloud video generation (choose one)

**Google Veo 2 (recommended):**
```bash
export GOOGLE_API_KEY=...    # or VERTEX_AI_PROJECT + GOOGLE_APPLICATION_CREDENTIALS
pip install google-genai
```

**RunwayML Gen-3:**
```bash
export RUNWAYML_API_KEY=...
pip install runwayml
```

**Kling AI:**
```bash
export KLING_API_KEY=...
```

---

## Running

```bash
# Default — 5 second cinematic landscape, prompt enhanced
python cookbook/56_text_to_video/run.py \
    --prompt "A duck walking through a quiet forest at dawn, soft morning light"

# No prompt enhancement (send raw prompt to video model)
python cookbook/56_text_to_video/run.py \
    --prompt "A mountain lake at sunset" \
    --enhance false

# Custom style and duration
python cookbook/56_text_to_video/run.py \
    --prompt "City traffic at night, neon reflections on wet pavement" \
    --style "cinematic, drone shot, slow motion" \
    --duration 8

# Portrait aspect (vertical video)
python cookbook/56_text_to_video/run.py \
    --prompt "A candle flickering in the wind" \
    --aspect portrait --duration 6

# Use RunwayML instead of Veo 2
python cookbook/56_text_to_video/run.py \
    --prompt "Ocean waves at golden hour" \
    --video-model runwayml/gen-3-alpha

# Custom output directory
python cookbook/56_text_to_video/run.py \
    --prompt "Snow falling on a quiet street" \
    --output-dir /tmp/videos
```

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@prompt` | TEXT | `A duck walking through a quiet forest at dawn...` | Base text prompt |
| `@style` | TEXT | `cinematic, realistic` | Visual style applied during enhancement |
| `@aspect` | TEXT | `landscape` | Aspect ratio: `landscape`, `portrait`, `square` |
| `@duration` | INT | `5` | Target clip duration in seconds |
| `@enhance` | BOOL | `TRUE` | Run gemma4 prompt enhancement before generation |
| `@video_model` | TEXT | `google/veo-2` | Video generation model |
| `@llm_model` | TEXT | `gemma4` | Local LLM for prompt enhancement |
| `@output_dir` | TEXT | `cookbook/56_text_to_video/outputs` | Directory to write generated video |
| `@log_dir` | TEXT | `cookbook/56_text_to_video/logs-spl` | Directory for log output |

---

## Output

`@video_path VIDEO` — file path of the generated video in `@output_dir`.

Filename pattern: `generated_<timestamp>.mp4`

---

## Composability

The generated video can be piped into downstream recipes:

```sql
-- Generate a video, then summarise it
CALL text_to_video(@prompt, ...) INTO @video
CALL video_summary(@video, 'summary', ...) INTO @summary

-- Generate a video, extract a frame, then caption it
CALL text_to_video(@prompt, ...) INTO @video
CALL video_to_image(@video, 'middle', ...) INTO @frame
CALL image_caption(@frame, 'detailed', ...) INTO @caption
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | API key missing or video model quota exceeded | Returns error string with `status = 'failed'` |
| `BudgetExceeded` | Token budget exceeded during prompt enhancement | Returns raw `@prompt` unenhanced with `status = 'budget_limit'` |

---

## Supported video models

| Model | Provider | API key env var | Notes |
|-------|----------|-----------------|-------|
| `google/veo-2` | Google DeepMind | `GOOGLE_API_KEY` | Default; highest quality |
| `runwayml/gen-3-alpha` | RunwayML | `RUNWAYML_API_KEY` | Good motion consistency |
| `kling/v1` | Kling AI | `KLING_API_KEY` | Cost-effective option |
