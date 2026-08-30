# Recipe 53: Text to Speech

**Category:** multimodal / generative  
**SPL version:** 3.0  
**Type:** TEXT → AUDIO  
**LLM required:** Optional — gemma4 (script prep, Ollama) + OpenAI TTS or system TTS  
**Demonstrates:** `AUDIO` as a first-class SPL output type, optional script prep pipeline, `EVALUATE @prep`

---

## What it does

Converts text to natural-sounding speech and saves it as an audio file.

```
[text input]
      │
      ▼  gemma4 via Ollama (optional — @prep = TRUE)
[cleaned, TTS-ready script]
      │
      ▼  OpenAI TTS (tts-1 / tts-1-hd) or system TTS (say / espeak)
[audio file saved to @output_dir]
```

The script prep step cleans markdown formatting, expands abbreviations, adds
natural pause punctuation, and replaces URLs — making the audio sound more
natural when the source text is AI-generated or technical content.

---

## Files

| File | Role |
|------|------|
| `text_to_speech.spl` | SPL logical view — 1 prompt function, 1 workflow |
| `run.py` | Physical runner — optional gemma4 prep + TTS API or system call |
| `outputs/` | Generated audio files written here |

---

## Prerequisites

```bash
# Cloud TTS (default)
export OPENAI_API_KEY=sk-...

# System TTS (no API key — fallback)
# macOS: built-in say command (no install needed)
# Linux: sudo apt install espeak

# For script prep (optional)
ollama serve
ollama pull gemma4:e4b
```

> **Note:** OpenAI TTS (`tts-1`, `tts-1-hd`) requires the official OpenAI API.
> Use `--backend system` to use `say` (macOS) or `espeak` (Linux) with no API key.

---

## Running

```bash
export OPENAI_API_KEY=sk-...
export OPENAI_BASE_URL="https://api.openai.com/v1"

# Basic TTS
python cookbook/55_text_to_speech/run.py \
    --text "SPL 3.0 multimodal support is ready for testing."

# Different voice
python cookbook/55_text_to_speech/run.py \
    --text "Welcome to the future of agentic workflows." \
    --voice nova

# HD quality
python cookbook/55_text_to_speech/run.py \
    --text "Natural language driven development is here." \
    --model tts-1-hd --voice alloy

# Expressive TTS with acting instructions (gpt-4o-mini-tts)
python cookbook/55_text_to_speech/run.py \
    --text "Breaking news: SPL 3.0 ships!" \
    --model gpt-4o-mini-tts --voice coral \
    --instructions "Speak like an excited news anchor."

# With script prep (cleans markdown, expands abbreviations)
python cookbook/55_text_to_speech/run.py \
    --text "**SPL v3.0** introduces e.g. IMAGE, AUDIO types. See https://example.com." \
    --prep --tone "professional"

# Read from file
python cookbook/55_text_to_speech/run.py \
    --file cookbook/50_code_pipeline/README.md \
    --prep --tone "conversational"

# System TTS (no API key needed)
python cookbook/55_text_to_speech/run.py \
    --text "Hello from SPL 3.0." \
    --backend system
```

---

## Parameters

### SPL workflow params

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@text` | TEXT | `Welcome to SPL 3.0...` | Source text to convert to speech |
| `@voice` | TEXT | `alloy` | OpenAI voice: `alloy`, `echo`, `fable`, `onyx`, `nova`, `shimmer` |
| `@tone` | TEXT | `neutral` | Tone for script prep: `neutral`, `professional`, `conversational` |
| `@model` | TEXT | `tts-1` | TTS model: `tts-1` (fast) or `tts-1-hd` (higher quality) |
| `@prep` | BOOL | `FALSE` | Run gemma4 script prep before TTS |
| `@output_dir` | TEXT | `cookbook/55_text_to_speech/outputs` | Directory to write audio file |
| `@llm_model` | TEXT | `gemma4:e4b` | Local LLM for script prep |
| `@log_dir` | TEXT | `cookbook/55_text_to_speech/logs-spl` | Directory for log output |

### CLI flags (run.py)

| Flag | Default | Description |
|------|---------|-------------|
| `--text` | *(required\*)* | Text to speak |
| `--file` | *(required\*)* | Read text from file (mutually exclusive with `--text`) |
| `--voice` | `alloy` | `alloy` \| `echo` \| `fable` \| `onyx` \| `nova` \| `shimmer` |
| `--model` | `tts-1` | `tts-1` \| `tts-1-hd` \| `gpt-4o-mini-tts` |
| `--instructions` | — | Speaking instructions (`gpt-4o-mini-tts` only) |
| `--prep` | off | Clean script via Gemma4 before TTS |
| `--tone` | `neutral` | Tone hint for script prep |
| `--backend` | `openai` | `openai` \| `system` (`say` / `espeak`) |
| `--output-dir` | `cookbook/55_text_to_speech/outputs` | Directory to write audio file |

---

## OpenAI voice options

| Voice | Character |
|-------|-----------|
| `alloy` | Neutral, balanced |
| `echo` | Male, clear |
| `fable` | British-accented, expressive |
| `onyx` | Deep, authoritative |
| `nova` | Female, warm |
| `shimmer` | Female, soft |

---

## Output

`@audio_path AUDIO` — file path of the generated audio in `@output_dir`.

Filename pattern: `speech_<timestamp>.mp3`

---

## Composability

```sql
-- Summarise audio, then read the summary aloud
CALL audio_summary(@clip, 'summary', 'concise paragraph', @model, 2048) INTO @summary
CALL text_to_speech(@summary, 'nova', 'professional', 'tts-1-hd', FALSE, @output_dir, @llm_model) INTO @audio

-- Generate code docs, then narrate them
CALL code_pipeline(@spec, ...) INTO @docs
CALL text_to_speech(@docs, 'alloy', 'neutral', 'tts-1', TRUE, @output_dir, @llm_model) INTO @narration
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | `OPENAI_API_KEY` missing or TTS model unavailable | Returns error string with `status = 'failed'`; try `--backend system` |
