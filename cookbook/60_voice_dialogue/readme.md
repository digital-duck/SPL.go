# Recipe 55: Voice Dialogue

**Category:** multimodal / conversational  
**SPL version:** 3.0  
**Type:** AUDIO + TEXT → TEXT + AUDIO  
**LLM required:** Yes — LFM-2.5 (OpenRouter, transcription) + gemma4 (Ollama, response) + OpenAI TTS or system TTS  
**Demonstrates:** Full audio-in / audio-out pipeline, three-model handoff, AUDIO as both input and output type

---

## What it does

A complete voice assistant pipeline: listens, understands, responds, and speaks.

```
[voice question — AUDIO]
      │
      ▼  LFM-2.5 via OpenRouter  (AUDIO → TEXT)
[verbatim transcript]
      │
      ▼  gemma4 via Ollama       (TEXT → TEXT)
[text response]
      │
      ▼  OpenAI TTS / espeak     (TEXT → AUDIO)
[spoken response — AUDIO file saved to @output_dir]
```

Three models, three modality hops, one declarative SPL workflow. The
orchestration logic contains no model-specific code — that lives in `run.py`
and the codec layer.

---

## Files

| File | Role |
|------|------|
| `voice_dialogue.spl` | SPL logical view — 2 prompt functions, 1 workflow |
| `run.py` | Physical runner — LFM-2.5 + gemma4 + TTS |
| `sample/` | Place audio question files here (`.wav` / `.mp3`) |
| `outputs/` | Spoken response audio files written here |

---

## Prerequisites

```bash
# Transcription (Step 1)
export OPENROUTER_API_KEY=sk-or-...

# LLM response (Step 2)
ollama serve
ollama pull gemma4:e4b

# TTS — spoken response (Step 3, choose one)
export OPENAI_API_KEY=sk-...   # OpenAI TTS (default)
# sudo apt install espeak      # or Linux system TTS (no API key)
# macOS say is built-in        # or macOS system TTS (no API key)
```

### Generate a test audio question

```bash
# Option A — OpenAI TTS
python -c "
import pathlib
from openai import OpenAI
client = OpenAI()
out = pathlib.Path('cookbook/60_voice_dialogue/sample/question.mp3')
out.parent.mkdir(parents=True, exist_ok=True)
response = client.audio.speech.create(
    model='tts-1', voice='alloy',
    input='What are the main features of SPL version 3 multimodal support?'
)
out.write_bytes(response.content)
print(f'audio saved: {out}')
"

# Option B — macOS say + ffmpeg
say -o cookbook/60_voice_dialogue/sample/question.aiff \
    "What is SPL 3.0 and why does it matter?"
ffmpeg -i cookbook/60_voice_dialogue/sample/question.aiff \
       cookbook/60_voice_dialogue/sample/question.wav

# Option C — espeak (Linux)
espeak "What are the main features of SPL 3.0?" \
    -w cookbook/60_voice_dialogue/sample/question.wav
```

---

## Running

```bash
export OPENROUTER_API_KEY=sk-or-...
export OPENAI_API_KEY=sk-...

# Minimal
python cookbook/60_voice_dialogue/run.py \
    --audio cookbook/60_voice_dialogue/sample/question.mp3

# Custom persona + context
python cookbook/60_voice_dialogue/run.py \
    --audio cookbook/60_voice_dialogue/sample/question.mp3 \
    --persona "a cheerful science teacher" \
    --context "SPL 3.0 is a multimodal agentic workflow language."

# HD voice response
python cookbook/60_voice_dialogue/run.py \
    --audio cookbook/60_voice_dialogue/sample/question.mp3 \
    --tts-voice nova --tts-model tts-1-hd

# Skip OpenAI TTS — use system espeak / say instead
python cookbook/60_voice_dialogue/run.py \
    --audio cookbook/60_voice_dialogue/sample/question.mp3 \
    --tts-backend system

# Different response model
python cookbook/60_voice_dialogue/run.py \
    --audio cookbook/60_voice_dialogue/sample/question.mp3 \
    --llm-model phi4:latest

# Custom output directory
python cookbook/60_voice_dialogue/run.py \
    --audio cookbook/60_voice_dialogue/sample/question.mp3 \
    --output-dir /tmp/voice_responses
```

---

## Parameters

### SPL workflow params

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@audio_in` | AUDIO | `cookbook/60_voice_dialogue/sample/question.wav` | Source audio question file |
| `@context` | TEXT | `''` | Optional context provided to the LLM responder |
| `@persona` | TEXT | `a helpful assistant` | Persona description for the LLM |
| `@asr_model` | TEXT | `liquid/lfm-2.5-1.2b-instruct:free` | Transcription model |
| `@asr_backend` | TEXT | `openrouter` | ASR backend: `openrouter` \| `ollama` |
| `@llm_model` | TEXT | `gemma4:e4b` | Ollama model for generating the text response |
| `@tts_voice` | TEXT | `alloy` | OpenAI TTS voice |
| `@tts_model` | TEXT | `tts-1` | OpenAI TTS model: `tts-1` or `tts-1-hd` |
| `@output_dir` | TEXT | `cookbook/60_voice_dialogue/outputs` | Directory to write response audio |
| `@log_dir` | TEXT | `cookbook/60_voice_dialogue/logs-spl` | Directory for log output |

### CLI flags (run.py)

| Flag | Default | Description |
|------|---------|-------------|
| `--audio` | *(required)* | Voice question WAV/MP3 file |
| `--context` | `""` | Text context to include in response |
| `--persona` | `"a helpful assistant"` | LLM response persona |
| `--asr-model` | `liquid/lfm-2.5-1.2b-instruct:free` | Transcription model |
| `--asr-backend` | `openrouter` | `openrouter` \| `ollama` |
| `--llm-model` | `gemma4:e4b` | Response LLM (Ollama) |
| `--tts-voice` | `alloy` | OpenAI TTS voice |
| `--tts-model` | `tts-1` | `tts-1` \| `tts-1-hd` \| `gpt-4o-mini-tts` |
| `--tts-backend` | `openai` | `openai` \| `system` (`say` / `espeak`) |
| `--output-dir` | `cookbook/60_voice_dialogue/outputs` | Directory to write response audio |

---

## Output

`@response TEXT` — the LLM's text response.

The spoken audio file is a side effect written to `@output_dir` by `run.py`.

Filename pattern: `response_<timestamp>.mp3`

---

## Composability

```sql
-- Convert audio format first if needed
CALL audio_convert(@raw_audio, 'mp3', '192k', 44100, @output_dir) INTO @audio
CALL voice_dialogue(@audio, @context, @persona, @llm_model, ...) INTO @response

-- Chain: extract video audio → voice dialogue
CALL video_to_audio(@video, 'mp3', '192k', 44100, @output_dir) INTO @audio
CALL voice_dialogue(@audio, @context, @persona, @llm_model, ...) INTO @response
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | Any of the three models unavailable | Returns error string with `status = 'failed'`; check each dependency |

**Dependency checklist if the pipeline fails:**

| Step | Model | Check |
|------|-------|-------|
| Transcription | LFM-2.5 (OpenRouter) | `OPENROUTER_API_KEY` set? |
| Response | gemma4 (Ollama) | `ollama serve` running? `gemma4:e4b` pulled? |
| TTS | OpenAI / system | `OPENAI_API_KEY` set? Or use `--tts-backend system` |
