# Recipe 50: Image Caption

**Category:** multimodal  
**SPL version:** 3.0  
**Type:** `IMAGE` → `TEXT`  
**LLM required:** Yes — gemma4 (native vision) via Ollama  
**Demonstrates:** `IMAGE` as a first-class SPL input type, multi-mode `EVALUATE`, vision model dispatch

Describe, analyse, or extract text from any image using a vision-capable model.

---

## How `photo IMAGE` travels to the LLM

```spl
CREATE FUNCTION caption(photo IMAGE, question TEXT)
RETURN TEXT
AS $$
You are a precise visual analyst.
...
Question: {question}
$$;
```

A `CREATE FUNCTION` in SPL is a **prompt template**, not procedural code. The body becomes the system prompt. The two parameters behave differently:

- `question TEXT` — string interpolation: `{question}` is replaced inline in the template.
- `photo IMAGE` — **not** interpolated as text. It is a typed signal to the runtime that an image must be attached as a separate multimodal content block alongside the rendered prompt.

The pipeline from file path to LLM has four stages:

```
.spl  @photo IMAGE (file path string)
        │
        ▼  executor.py — no coercion, passes through as-is
        │  (INT/FLOAT get coerced; IMAGE/AUDIO/VIDEO do not)
        │
        ▼  spl3/codecs/image_codec.encode_image()
        │  • PIL open → resize to ≤1568px longest edge
        │  • JPEG compress at quality 85
        │  • base64-encode → ImagePart dict
        │    {"type": "image", "source": "base64",
        │     "media_type": "image/jpeg", "data": "<b64>"}
        │  (URL strings are passed through unchanged — no download)
        │
        ▼  adapter.generate_multimodal([ImagePart, TextPart])
        │  content = [
        │    {"type": "image", "source": "base64", ...},
        │    {"type": "text",  "text": "<rendered prompt>"}
        │  ]
        │
        ▼  LLM API (e.g. Ollama gemma4) — sees image bytes + text prompt
```

Key design point: `photo IMAGE` bypasses the template substitution system entirely. The image is encoded and forwarded as a separate content block, not as text inserted into the prompt.

---

## Modes

| Mode | Prompt strategy | Best for |
|------|----------------|----------|
| `caption` | Q&A — answer a specific question about the image | Targeted queries |
| `detailed` | Full description: subjects, setting, colours, text, mood | Scene understanding, accessibility |
| `ocr` | Extract all visible text exactly as it appears | Screenshots, documents, signs |

---

## Files

| File | Role |
|------|------|
| `image_caption.spl` | SPL logical view — 3 prompt functions, 1 workflow |
| `run.py` | Physical runner — image codec + multimodal adapter call |
| `sample/` | Place test images here (`.jpg` / `.png` recommended) |

---

## Prerequisites

```bash
ollama serve
ollama pull gemma4:e4b       # vision model
pip install Pillow           # image encoding (strongly recommended)
```

---

## Testing

### Generate a test image (no download needed)

```bash
python -c "
from PIL import Image, ImageDraw
import pathlib
pathlib.Path('cookbook/51_image_caption/sample').mkdir(parents=True, exist_ok=True)
img = Image.new('RGB', (800, 600), (70, 130, 180))
draw = ImageDraw.Draw(img)
draw.polygon([(0,600),(200,250),(400,600)], fill=(90,90,90))
draw.polygon([(150,600),(380,200),(600,600)], fill=(110,110,110))
draw.ellipse([600,50,720,170], fill=(255,220,50))
img.save('cookbook/51_image_caption/sample/photo.jpg', 'JPEG')
print('test image created')
"
```

### Run via `run.py` (current physical runner)

`run.py` directly invokes the codec layer and `generate_multimodal()`, mirroring what `image_caption.spl` declares. Use this until `spl-go run` gains native multimodal param dispatch.

```bash
# Default — caption mode, default question
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg

# Detailed description
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg \
    --mode detailed

# OCR — extract visible text
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg \
    --mode ocr

# Ask a specific question
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg \
    --question "What colours are dominant in this scene?"

# Caption a public URL (no local encoding)
python cookbook/51_image_caption/run.py \
    --image "https://upload.wikimedia.org/wikipedia/commons/thumb/4/47/PNG_transparency_demonstration_1.png/280px-PNG_transparency_demonstration_1.png"

# Larger model for higher accuracy
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg \
    --model gemma4:e2b --max-dim 1568

# Larger output budget
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg \
    --mode detailed --max-tokens 2048
```

### Via `spl-go run` (native SPL executor)

```bash
spl-go run cookbook/51_image_caption/image_caption.spl \
    --adapter ollama \
    --param photo="cookbook/51_image_caption/sample/photo.jpg" \
    --param model="gemma4:e4b"

# Detailed mode
spl-go run cookbook/51_image_caption/image_caption.spl \
    --adapter ollama \
    --param photo="cookbook/51_image_caption/sample/asian-boy-and-girl-standing.png" \
    --param mode="detailed" \
    --param model="gemma4:e4b"

# OCR
spl-go run cookbook/51_image_caption/image_caption.spl \
    --adapter ollama \
    --param photo="cookbook/51_image_caption/sample/photo.jpg" \
    --param mode="ocr" \
    --param model="gemma4:e4b"
```

### Via OpenRouter (no local GPU needed)

```bash
export OPENROUTER_API_KEY=sk-...
python cookbook/51_image_caption/run.py \
    --image cookbook/51_image_caption/sample/photo.jpg \
    --backend openrouter \
    --model liquid/lfm-2.5-1.2b-instruct:free
```

---

## Parameters

### `run.py` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--image` | *(required)* | Local file path or public HTTPS URL |
| `--question` | `"What is in this image? Describe it in detail."` | Question (caption mode only) |
| `--mode` | `caption` | `caption` \| `detailed` \| `ocr` |
| `--model` | `gemma4` | Any vision-capable Ollama model |
| `--backend` | `ollama` | `ollama` (local) or `openrouter` (cloud) |
| `--max-dim` | `1024` | Resize longest edge to N pixels before encoding |
| `--max-tokens` | `1024` | Max output tokens |

### SPL workflow inputs (`image_caption.spl`)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@photo` | IMAGE | `cookbook/51_image_caption/sample/photo.jpg` | Source image file path |
| `@question` | TEXT | `What is in this image? Describe it in detail.` | Question for `caption` mode |
| `@mode` | TEXT | `caption` | Output mode: `caption`, `detailed`, `ocr` |
| `@model` | TEXT | `gemma4:e4b` | Vision model (Ollama) |
| `@output_budget` | INT | `1024` | Max output tokens |
| `@log_dir` | TEXT | `cookbook/51_image_caption/logs-spl` | Directory for log output |

---

## Supported models

| Model | Pull command | Notes |
|-------|-------------|-------|
| `gemma4:e4b` | `ollama pull gemma4:e4b` | Recommended — native multimodal |
| `gemma4:e2b` | `ollama pull gemma4:e2b` | Higher accuracy |
| `llava:13b` | `ollama pull llava:13b` | Fallback if Gemma 4 not available |
| `llava-phi3` | `ollama pull llava-phi3` | Lightweight fallback |
| `liquid/lfm-2.5-1.2b-instruct:free` | OpenRouter | No local GPU needed |

---

## Output

`@result TEXT` — the image description, answer, or OCR text.

---

## Composability

```sql
-- Convert format first, then caption
CALL image_convert(@raw_image, 'jpeg', 85, @output_dir) INTO @image
CALL image_caption(@image, @question, 'caption', @model, @output_budget) INTO @caption

-- Extract video frame, then caption
CALL video_to_image(@video, 'middle', ...) INTO @frame
CALL image_caption(@frame, '', 'detailed', @model, 2048) INTO @description
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `ModelUnavailable` | Ollama not running or gemma4 not pulled | Returns error string with `status = 'failed'` |
