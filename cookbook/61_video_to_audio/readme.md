# Recipe 61: Video to Audio

**Category:** media / codec  
**SPL version:** 3.0  
**Type:** VIDEO → AUDIO  
**LLM required:** No — deterministic codec operation  
**Demonstrates:** VIDEO as SPL input type, `CALL` built-in tool, composable media pipeline utility

---

## What it does

Extracts the audio track from a video file and saves it as a standalone audio file.

Common use cases:
- Extract speech from a recorded meeting (`.mp4`) for transcription via recipe 52 (`audio_summary`)
- Archive audio from video content as FLAC (lossless)
- Prepare audio for recipe 55 (`text_to_speech`) comparison or recipe 60 (`voice_dialogue`)

No LLM is involved — ffmpeg handles the demux and codec transcode in a single pass.

---

## Files

| File | Role |
|------|------|
| `video_to_audio.spl` | SPL logical view — declarative workflow definition |
| `run.py` | Physical runner — ffmpeg audio extraction |
| `sample/` | Place source video files here (`.mp4` recommended) |
| `outputs/` | Extracted audio files written here |

---

## Prerequisites

```bash
pip install pydub
sudo apt install ffmpeg      # Ubuntu / Debian
# brew install ffmpeg        # macOS

ffmpeg -version              # verify
```

---

## Running

```bash
# Default — extract as MP3 at 192k
python cookbook/61_video_to_audio/run.py \
    --video cookbook/61_video_to_audio/sample/clip.mp4

# Extract as WAV (lossless, large)
python cookbook/61_video_to_audio/run.py \
    --video cookbook/61_video_to_audio/sample/clip.mp4 \
    --target-format wav --sample-rate 48000

# Extract as FLAC (lossless compressed, archival)
python cookbook/61_video_to_audio/run.py \
    --video cookbook/61_video_to_audio/sample/clip.mp4 \
    --target-format flac

# High-quality MP3
python cookbook/61_video_to_audio/run.py \
    --video cookbook/61_video_to_audio/sample/clip.mp4 \
    --target-format mp3 --bitrate 320k

# Custom output directory
python cookbook/61_video_to_audio/run.py \
    --video cookbook/61_video_to_audio/sample/clip.mp4 \
    --output-dir /tmp/audio
```

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@video` | VIDEO | `cookbook/61_video_to_audio/sample/clip.mp4` | Source video file path |
| `@target_format` | TEXT | `mp3` | Output audio format: `mp3`, `wav`, `flac`, `ogg`, `m4a` |
| `@bitrate` | TEXT | `192k` | Bitrate for lossy formats (e.g. `128k`, `192k`, `320k`) |
| `@sample_rate` | INT | `44100` | Output sample rate in Hz |
| `@output_dir` | TEXT | `cookbook/61_video_to_audio/outputs` | Directory to write extracted audio |
| `@log_dir` | TEXT | `cookbook/61_video_to_audio/logs-spl` | Directory for log output |

---

## Output

`@audio AUDIO` — file path of the extracted audio in `@output_dir`.

Filename pattern: `<video_stem>.<target_format>` (e.g. `clip.mp3`).

---

## Composability

`video_to_audio` is the natural first step in a video → text pipeline:

```sql
-- Full pipeline: video → audio → transcript
CALL video_to_audio(@video, 'mp3', '192k', 44100, @output_dir) INTO @audio
CALL audio_summary(@audio, 'transcript', '', @model, 4096) INTO @transcript

-- Or: video → audio → summary
CALL video_to_audio(@video, 'wav', '192k', 44100, @output_dir) INTO @audio
CALL audio_summary(@audio, 'summary', 'concise paragraph', @model, 2048) INTO @summary
```

This is also useful when `video_summary` (recipe 53) is not yet available for
a given model or runtime — audio extraction + audio summary is a reliable fallback.

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `FileNotFound` | Video file path does not exist | Returns error string with `status = 'failed'` |
| `NoAudioTrack` | Video file contains no audio stream | Returns error string with `status = 'failed'` |
| `CodecError` | ffmpeg not installed or codec failure | Returns error string with `status = 'failed'`; install ffmpeg |
