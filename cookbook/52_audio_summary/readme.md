# Recipe 51: Audio Summary

**Category:** multimodal  
**SPL version:** 3.0  
**Type:** `AUDIO` → `TEXT`  
**LLM required:** Yes — gemma4 via Ollama (confirmed); LFM-2.5 via OpenRouter (confirmed)  
**Demonstrates:** `AUDIO` as a first-class SPL input type, multi-mode `EVALUATE`, speech-capable model dispatch

Transcribe, summarise, or extract key points from an audio clip.

---

## How `clip AUDIO` travels to the LLM

The pipeline mirrors Recipe 50's IMAGE flow — the same four-stage pattern applies:

```
.spl  @clip AUDIO (file path string)
        │
        ▼  executor.py — no coercion, passes through as-is
        │
        ▼  spl3/codecs/audio_codec.encode_audio()
        │  • reads WAV/MP3/OGG/FLAC bytes from disk
        │  • optional --to-wav conversion via pydub
        │  • base64-encode → AudioPart dict
        │    {"type": "audio", "source": "base64",
        │     "media_type": "audio/wav", "data": "<b64>"}
        │
        ▼  adapter.generate_multimodal([TextPart, AudioPart])
        │  OpenAI input_audio format:
        │  {"type": "input_audio", "input_audio": {"data": "...", "format": "wav"}}
        │
        ▼  LFM-2.5 (OpenRouter) — native audio understanding
```

`clip AUDIO` is not interpolated into the prompt text. It is encoded and forwarded as a separate content block alongside the rendered prompt.

---

## Modes

| Mode | What it produces | Best for |
|------|-----------------|----------|
| `summary` | Concise prose summary in a chosen style | Meetings, lectures, podcasts |
| `transcribe` | Verbatim transcript with speaker changes on new lines | Documentation, archives |
| `key_points` | Structured Markdown: topic, key points, decisions, action items | Meeting notes |

---

## Files

| File | Role |
|------|------|
| `audio_summary.spl` | SPL logical view — 3 prompt functions, 1 workflow |
| `run.py` | Physical runner — audio codec + LiquidAdapter multimodal call |
| `sample/` | Place test audio files here (`.wav` / `.mp3`) |

---

## Prerequisites

```bash
export OPENROUTER_API_KEY=sk-or-...
```

LFM-2.5 on OpenRouter is free-tier: `liquid/lfm-2.5-1.2b-instruct:free`.

Optional — for MP3/OGG → WAV conversion:

```bash
pip install pydub
sudo apt install ffmpeg      # Ubuntu / Debian
```

---

## Generate a test clip

```bash
# Linux — espeak-ng (no API key needed)
espeak-ng "Welcome to the SPL 3.0 multimodal demo. Today we test audio summarisation." \
    -w cookbook/52_audio_summary/sample/clip.wav

# macOS — say + ffmpeg
say -o cookbook/52_audio_summary/sample/clip.aiff \
    "Welcome to the SPL 3.0 multimodal demo."
ffmpeg -i cookbook/52_audio_summary/sample/clip.aiff \
       cookbook/52_audio_summary/sample/clip.wav

# OpenAI TTS (requires OPENAI_API_KEY)
python -c "
import pathlib
from openai import OpenAI
client = OpenAI()
out = pathlib.Path('cookbook/52_audio_summary/sample/clip.mp3')
out.parent.mkdir(parents=True, exist_ok=True)
response = client.audio.speech.create(
    model='tts-1', voice='alloy',
    input='What are the main features of SPL version 3 multimodal support?'
)
out.write_bytes(response.content)
print(f'audio saved: {out}')
"
```

---

## Testing

### Via `spl-go run` (native SPL executor)

```bash
export OPENROUTER_API_KEY=sk-or-...

# Default — summary mode
spl-go run cookbook/52_audio_summary/audio_summary.spl \
    --adapter ollama \
    --param clip="cookbook/52_audio_summary/sample/harvard.wav" \
    --param model="gemma4:e4b"

# Verbatim transcript
spl-go run cookbook/52_audio_summary/audio_summary.spl \
    --adapter ollama \
    --param model="gemma4:e4b" \
    --param clip="cookbook/52_audio_summary/sample/short-conversation.mp3" \
    --param mode="transcribe"


# Key points (meeting notes style)
spl-go run cookbook/52_audio_summary/audio_summary.spl \
    --adapter openrouter \
    --param clip="cookbook/52_audio_summary/sample/harvard.wav" \
    --param mode="key_points"

# Custom summary style
spl-go run cookbook/52_audio_summary/audio_summary.spl \
    --adapter openrouter \
    --param clip="cookbook/52_audio_summary/sample/harvard.wav" \
    --param style="three bullet points, executive summary"
```

### Via `run.py` (physical runner with extra options)

```bash
export OPENROUTER_API_KEY=sk-or-...

# Summary (default)
python cookbook/52_audio_summary/run.py \
    --audio cookbook/52_audio_summary/sample/clip.wav

# Transcribe
python cookbook/52_audio_summary/run.py \
    --audio cookbook/52_audio_summary/sample/clip.wav \
    --mode transcribe

# Key points
python cookbook/52_audio_summary/run.py \
    --audio cookbook/52_audio_summary/sample/clip.wav \
    --mode key_points

# Custom style
python cookbook/52_audio_summary/run.py \
    --audio cookbook/52_audio_summary/sample/clip.wav \
    --style "three bullet points, executive summary"

# Convert MP3 to WAV before sending
python cookbook/52_audio_summary/run.py \
    --audio podcast.mp3 --to-wav

# Larger output budget for long recordings
python cookbook/52_audio_summary/run.py \
    --audio meeting.wav --mode transcribe --max-tokens 4096
```

> **Ollama:** gemma4 via Ollama has confirmed audio support. Use `--adapter ollama --param model="gemma4:e4b"` for local inference with no API key required.

---

## Parameters

### `spl-go run` / SPL workflow inputs

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@clip` | AUDIO | `cookbook/52_audio_summary/sample/clip.wav` | Source audio file path |
| `@mode` | TEXT | `summary` | Output mode: `summary`, `transcribe`, `key_points` |
| `@style` | TEXT | `concise paragraph` | Prose style for `summary` mode |
| `@model` | TEXT | `liquid/lfm-2.5-1.2b-instruct:free` | Speech-capable model (OpenRouter) |
| `@output_budget` | INT | `2048` | Max output tokens |
| `@log_dir` | TEXT | `cookbook/52_audio_summary/logs-spl` | Log directory |

### `run.py` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--audio` | *(required)* | Path to audio file (WAV, MP3, OGG, FLAC) |
| `--mode` | `summary` | `summary` \| `transcribe` \| `key_points` |
| `--style` | `"concise paragraph"` | Style hint for summary mode |
| `--backend` | `openrouter` | `openrouter` or `ollama` (both confirmed) |
| `--model` | *(auto)* | Override model name |
| `--to-wav` | off | Convert to WAV via pydub before encoding |
| `--max-tokens` | `2048` | Max output tokens |

---

## Audio format notes

| Format | Encode as-is | Requires pydub |
|--------|-------------|----------------|
| `.wav` | yes | no |
| `.mp3` | yes (audio/mp3) | only for `--to-wav` |
| `.ogg` | yes (audio/ogg) | only for `--to-wav` |
| `.flac` | yes (audio/flac) | only for `--to-wav` |

If you get format errors from the API, add `--to-wav` to normalise to WAV first.

---

## Output

`@result TEXT` — transcript, summary, or key points in the format matching `@mode`.

---

## Composability

```sql
-- Extract audio from video, then summarise
CALL video_to_audio(@video, 'mp3', '192k', 44100, @output_dir) INTO @audio
CALL audio_summary(@audio, 'summary', 'concise paragraph', @model, 2048) INTO @summary

-- Transcribe first, then summarise the transcript as text
CALL audio_summary(@clip, 'transcribe', '', @model, 4096) INTO @transcript
CALL text_summary(@transcript, 'key points') INTO @summary
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | `OPENROUTER_API_KEY` not set or LFM-2.5 unavailable | Returns error string with `status = 'failed'` |


## Sample audio file

- https://homepage.ntu.edu.tw/~karchung/miniconversations/MC.htm

---

## Installing espeak-ng on Ubuntu

On modern Ubuntu the maintained package is `espeak-ng` (the fork of the original `espeak` — same CLI flags, better voice quality):

```bash
sudo apt install espeak-ng
```

Then generate a test clip:

```bash
espeak-ng "Welcome to the SPL 3.0 multimodal demo. Today we test audio summarisation." \
    -w cookbook/52_audio_summary/sample/clip.wav
```