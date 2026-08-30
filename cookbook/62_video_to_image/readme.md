# Recipe 62: Video to Image

**Category:** media / codec + optional multimodal  
**SPL version:** 3.0  
**Type:** VIDEO → IMAGE  
**LLM required:** Optional — gemma4 vision for frame captioning (`@caption = TRUE`)  
**Demonstrates:** VIDEO input type, codec + optional LLM hybrid, `EVALUATE @mode`, `EVALUATE @caption`

---

## What it does

Extracts one or more frames from a video file as image files, with optional
gemma4 vision captioning of the extracted frame.

This is a two-stage pipeline:

```
[video file]
      │
      ▼  ffmpeg (codec layer)
[extracted frame as IMAGE]
      │  (if @caption = TRUE)
      ▼  gemma4 vision (optional)
[frame caption TEXT logged]
```

The extracted frame can be passed directly to recipe 50 (`image_caption`)
or recipe 54 (`image_restyle`) for further processing.

---

## Files

| File | Role |
|------|------|
| `video_to_image.spl` | SPL logical view — codec extraction + optional caption |
| `run.py` | Physical runner — ffmpeg frame extraction + optional Ollama vision |
| `sample/` | Place source video files here (`.mp4` recommended) |
| `outputs/` | Extracted frame images written here |

---

## Prerequisites

```bash
# Required — frame extraction
sudo apt install ffmpeg      # Ubuntu / Debian
# brew install ffmpeg        # macOS

# Optional — frame captioning
ollama serve
ollama pull gemma4
```

---

## Running

```bash
# Extract middle frame (default)
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4

# First frame
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4 \
    --mode first

# Last frame
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4 \
    --mode last

# Specific timestamp (HH:MM:SS)
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4 \
    --mode timestamp --timestamp 00:00:05

# Extract + caption with gemma4 vision
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4 \
    --caption

# Extract + caption with context hint
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4 \
    --mode timestamp --timestamp 00:00:10 \
    --caption \
    --context "This is a wildlife documentary about migratory birds"

# Custom output directory
python cookbook/62_video_to_image/run.py \
    --video cookbook/62_video_to_image/sample/clip.mp4 \
    --output-dir /tmp/frames
```

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@video` | VIDEO | `cookbook/62_video_to_image/sample/clip.mp4` | Source video file path |
| `@mode` | TEXT | `middle` | Extraction mode: `first`, `middle`, `last`, `timestamp` |
| `@timestamp` | TEXT | `00:00:05` | Timestamp for `timestamp` mode (format `HH:MM:SS`) |
| `@fps` | FLOAT | `1.0` | Frame rate for `sample` mode (frames per second) |
| `@max_frames` | INT | `10` | Maximum frames for `sample` mode |
| `@caption` | BOOL | `FALSE` | Run gemma4 vision captioning on the extracted frame |
| `@context` | TEXT | `''` | Optional context hint passed to the caption model |
| `@model` | TEXT | `gemma4` | Vision model for captioning (Ollama) |
| `@output_dir` | TEXT | `cookbook/62_video_to_image/outputs` | Directory to write extracted frame(s) |
| `@log_dir` | TEXT | `cookbook/62_video_to_image/logs-spl` | Directory for log output |

---

## Extraction modes

| Mode | Description | Use case |
|------|-------------|----------|
| `first` | Frame at t=0 | Thumbnail, cover image |
| `middle` | Frame at video midpoint | Representative scene |
| `last` | Final frame | End state, result |
| `timestamp` | Frame at exact `@timestamp` | Known moment of interest |
| `sample` | `@max_frames` evenly spaced frames at `@fps` | Storyboard, timeline overview |

---

## Output

`@frame IMAGE` — file path of the extracted frame in `@output_dir`.

Filename pattern: `frame_<mode>_<timestamp>.jpg` (e.g. `frame_middle_00:00:07.jpg`).

If `@caption = TRUE`, the caption is logged at INFO level and embedded in the
workflow output metadata (accessible via `RETURN ... WITH caption = @frame_caption`).

---

## Composability

```sql
-- Extract a frame then do full image analysis
CALL video_to_image(@video, 'middle', ...) INTO @frame
CALL image_caption(@frame, 'detailed', @model, @output_budget) INTO @caption

-- Restyle a key video frame
CALL video_to_image(@video, 'timestamp', '00:00:10', ...) INTO @frame
CALL image_restyle(@frame, 'oil painting, impressionist', ...) INTO @restyled

-- Storyboard: extract multiple frames then caption each
-- (CALL PARALLEL across frames in an orchestrator workflow)
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `FileNotFound` | Video file path does not exist | Returns error string with `status = 'failed'` |
| `InvalidTimestamp` | `@timestamp` exceeds video duration | Returns error string with `status = 'failed'` |
| `ModelUnavailable` | Ollama not running (caption mode only) | Returns frame path without caption; `status = 'partial'` — not a crash |
