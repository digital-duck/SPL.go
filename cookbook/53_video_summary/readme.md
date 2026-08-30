# Recipe 60: Video Summary

**Category:** multimodal / understanding  
**SPL version:** 3.0  
**Type:** VIDEO → TEXT  
**LLM required:** Yes — gemma4 (native multimodal: text + vision + audio)  
**Demonstrates:** VIDEO as a first-class SPL input type, multi-mode `EVALUATE`, gemma4 video understanding

---

## What it does

Analyses a video clip and produces a text output — summary, verbatim transcript,
key moments list, or chapter breakdown — using gemma4's native multimodal capability
(vision + audio in a single model call).

**NOTE:** `VIDEO` is a first-class SPL 3.0 type (`SPL3Type.VIDEO`).
This is the first recipe in the SPL30 cookbook with `VIDEO` as input.

The codec layer (`spl/codecs/video_codec.py`) extracts frames and audio from the
video file and encodes them before passing to `generate_multimodal()`. The `.spl`
logical view remains hardware-agnostic.

---

## Files

| File | Role |
|------|------|
| `video_summary.spl` | SPL logical view — 4 prompt functions, 1 workflow |
| `run.py` | Physical runner — ffmpeg frame extraction + Ollama multimodal call |
| `sample/` | Place test video clips here (`.mp4` recommended) |

---

## Prerequisites

```bash
ollama serve
ollama pull gemma4           # native multimodal — text + vision + audio

sudo apt install ffmpeg      # frame and audio extraction
# brew install ffmpeg        # macOS
```

---

## Running

```bash
# Default — concise paragraph summary
python cookbook/53_video_summary/run.py \
    --video cookbook/53_video_summary/sample/clip.mp4

# Verbatim transcript with timestamps
python cookbook/53_video_summary/run.py \
    --video cookbook/53_video_summary/sample/clip.mp4 \
    --mode transcript

# Structured key moments list
python cookbook/53_video_summary/run.py \
    --video cookbook/53_video_summary/sample/clip.mp4 \
    --mode key_moments

# Chapter-style breakdown with titles
python cookbook/53_video_summary/run.py \
    --video cookbook/53_video_summary/sample/clip.mp4 \
    --mode chapters

# Custom summary style
python cookbook/53_video_summary/run.py \
    --video cookbook/53_video_summary/sample/clip.mp4 \
    --mode summary --style "three bullet points, executive style"

# Larger output budget for long videos
python cookbook/53_video_summary/run.py \
    --video cookbook/53_video_summary/sample/clip.mp4 \
    --mode transcript --output-budget 4096
```

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@clip` | VIDEO | `cookbook/53_video_summary/sample/clip.mp4` | Source video file path |
| `@mode` | TEXT | `summary` | Output mode: `summary`, `transcript`, `key_moments`, `chapters` |
| `@style` | TEXT | `concise paragraph` | Prose style for `summary` mode |
| `@model` | TEXT | `gemma4` | Multimodal LLM for video understanding |
| `@output_budget` | INT | `2048` | Max output tokens |
| `@log_dir` | TEXT | `cookbook/53_video_summary/logs-spl` | Directory for log output |

---

## Output modes

| Mode | Output format | Best for |
|------|--------------|----------|
| `summary` | Prose paragraph(s) | Quick understanding of video content |
| `transcript` | `[MM:SS] Speaker: text` | Meeting recordings, interviews, lectures |
| `key_moments` | Numbered timestamp list | Sports, events, tutorial videos |
| `chapters` | Markdown with titles + descriptions | Long-form content, courses, documentaries |

---

## Output

`@result TEXT` — the analysis in the format corresponding to `@mode`.

---

## Composability

```sql
-- Extract audio first (if gemma4 video support is limited), then summarise
CALL video_to_audio(@clip, 'mp3', '192k', 44100, @output_dir) INTO @audio
CALL audio_summary(@audio, 'summary', @style, @model, @output_budget) INTO @result

-- Extract a key frame and caption it alongside the summary
CALL video_to_image(@clip, 'middle', ...) INTO @frame
CALL image_caption(@frame, 'detailed', @model) INTO @frame_caption
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | Ollama not running or gemma4 not pulled | Returns error string with `status = 'failed'` |
| `FileNotFound` | Video file path does not exist | Returns error string with `status = 'failed'` |
