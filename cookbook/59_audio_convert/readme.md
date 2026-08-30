# Recipe 59: Audio Format Conversion

**Category:** media / codec  
**SPL version:** 3.0  
**Type:** AUDIO → AUDIO  
**LLM required:** No — deterministic codec operation  
**Demonstrates:** `CALL` built-in tool, `EXCEPTION WHEN` for codec errors, composable media utility

---

## What it does

Converts an audio file between formats using pydub and ffmpeg.
No LLM is involved — this is a deterministic operation.

As a self-contained SPL `WORKFLOW`, it is callable via `CALL audio_convert(...)` from
any orchestrator — for example, normalising audio to WAV before transcription
(recipe 52 `audio_summary`) or converting TTS output to a different format
after recipe 55 (`text_to_speech`).

**Supported formats:**

| Format | Extension | Notes |
|--------|-----------|-------|
| MP3 | `.mp3` | Lossy; `@bitrate` controls quality (e.g. `128k`, `192k`, `320k`) |
| WAV | `.wav` | Lossless PCM; large files; universal compatibility |
| OGG | `.ogg` | Open lossy format; good quality/size ratio |
| FLAC | `.flac` | Lossless compression; archival quality |
| M4A | `.m4a` | AAC container; Apple ecosystem default |

---

## Files

| File | Role |
|------|------|
| `audio_convert.spl` | SPL logical view — declarative workflow definition |
| `run.py` | Physical runner — pydub + ffmpeg conversion |
| `sample/` | Place source audio files here for testing |
| `outputs/` | Converted audio files written here |

---

## Prerequisites

```bash
sudo apt install ffmpeg      # Ubuntu / Debian
# brew install ffmpeg        # macOS

ffmpeg -version              # verify
```

> `pydub` is listed in the `.spl` comments for context but `run.py` calls
> ffmpeg directly — no Python audio library dependency needed.

### Get a sample audio file

```bash
# Option A — reuse clip from recipe 52 (if already present)
cp cookbook/52_audio_summary/sample/clip.mp3 \
   cookbook/59_audio_convert/sample/clip.mp3

# Option B — generate one with espeak (Linux)
espeak "Audio format conversion test for SPL 3.0." \
    -w cookbook/59_audio_convert/sample/clip.wav

# Option C — generate one with OpenAI TTS
python -c "
from openai import OpenAI; import pathlib
out = pathlib.Path('cookbook/59_audio_convert/sample/clip.mp3')
out.parent.mkdir(parents=True, exist_ok=True)
r = OpenAI().audio.speech.create(model='tts-1', voice='alloy',
    input='Audio format conversion test for SPL 3.0.')
out.write_bytes(r.content); print('saved:', out)
"
```

---

## Testing — run.py (physical runner)

`run.py` calls ffmpeg directly. No LLM, no SPL executor needed.

```bash
# MP3 → WAV
python cookbook/59_audio_convert/run.py \
    --audio cookbook/59_audio_convert/sample/clip.mp3 \
    --target-format wav

# MP3 → OGG (128k bitrate)
python cookbook/59_audio_convert/run.py \
    --audio cookbook/59_audio_convert/sample/clip.mp3 \
    --target-format ogg --bitrate 128k

# MP3 → FLAC (lossless archival)
python cookbook/59_audio_convert/run.py \
    --audio cookbook/59_audio_convert/sample/clip.mp3 \
    --target-format flac

# WAV → MP3 (high quality)
python cookbook/59_audio_convert/run.py \
    --audio cookbook/59_audio_convert/sample/clip.wav \
    --target-format mp3 --bitrate 320k --sample-rate 48000

# Custom output directory
python cookbook/59_audio_convert/run.py \
    --audio cookbook/59_audio_convert/sample/clip.mp3 \
    --target-format wav --output-dir /tmp/converted
```

---

## Testing — SPL executor (spl run)

`spl run` exercises the `.spl` workflow through the SPL executor.
The `convert_audio` tool must be loaded from `cookbook/tools/registry.py`.

```bash
# Default params (WAV → MP3, 192k)
spl-go run cookbook/59_audio_convert/audio_convert.spl \
    --tools cookbook/tools/registry.py

# MP3 → OGG
spl-go run cookbook/59_audio_convert/audio_convert.spl \
    --tools cookbook/tools/registry.py \
    -p audio=cookbook/59_audio_convert/sample/clip.mp3 \
    -p target_format=ogg \
    -p bitrate=128k

# MP3 → FLAC, custom output dir
spl-go run cookbook/59_audio_convert/audio_convert.spl \
    --tools cookbook/tools/registry.py \
    -p audio=cookbook/59_audio_convert/sample/clip.mp3 \
    -p target_format=flac \
    -p output_dir=/tmp/converted
```

> No `--adapter` needed — this workflow has no `GENERATE` statements.

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@audio` | AUDIO | `cookbook/59_audio_convert/sample/clip.wav` | Source audio file path |
| `@target_format` | TEXT | `mp3` | Target format: `mp3`, `wav`, `ogg`, `flac`, `m4a` |
| `@bitrate` | TEXT | `192k` | Bitrate for lossy formats (e.g. `128k`, `192k`, `320k`) |
| `@sample_rate` | INT | `44100` | Output sample rate in Hz (e.g. `44100`, `48000`) |
| `@output_dir` | TEXT | `cookbook/59_audio_convert/outputs` | Directory to write converted file |
| `@log_dir` | TEXT | `cookbook/59_audio_convert/logs-spl` | Directory for log output |

---

## Output

`@converted AUDIO` — file path of the converted audio in `@output_dir`.

Filename pattern: `<original_stem>.<target_format>` (e.g. `clip.mp3`).

---

## Composability

`audio_convert` is designed to be composed with other recipes via `CALL`:

```sql
-- Normalise format before summarisation
CALL audio_convert(@raw_audio, 'wav', '192k', 44100, @output_dir) INTO @audio
CALL audio_summary(@audio, 'transcribe', @style, @model, @output_budget) INTO @transcript
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `FileNotFound` | Source audio path does not exist | Returns error string with `status = 'failed'` |
| `UnsupportedFormat` | Target format not supported | Returns error string with `status = 'failed'` |
| `CodecError` | ffmpeg not installed or codec failure | Returns error string with `status = 'failed'`; install ffmpeg |
