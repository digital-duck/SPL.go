# Recipe 57: Image Format Conversion

**Category:** media / codec  
**SPL version:** 3.0  
**Type:** IMAGE → IMAGE  
**LLM required:** No — deterministic codec operation  
**Demonstrates:** `CALL` built-in tool, `EXCEPTION WHEN` for codec errors, composable media utility

---

## What it does

Converts an image file between formats using the Pillow codec layer.
No LLM is involved — this is a deterministic operation that completes in milliseconds.

As a self-contained SPL `WORKFLOW`, it is callable via `CALL image_convert(...)` from
any orchestrator workflow — for example, normalising all input images to JPEG before
passing them to recipe 50 (`image_caption`) or recipe 54 (`image_restyle`).

**Supported formats:**

| Format | Extension | Notes |
|--------|-----------|-------|
| JPEG | `.jpg` / `.jpeg` | Lossy; `@quality` controls compression (1–95) |
| PNG | `.png` | Lossless |
| WebP | `.webp` | Lossy or lossless; best web compression |
| BMP | `.bmp` | Uncompressed raster |
| GIF | `.gif` | Lossless; palette-limited (256 colours) |

---

## Files

| File | Role |
|------|------|
| `image_convert.spl` | SPL logical view — declarative workflow definition |
| `run.py` | Physical runner — Pillow-based format conversion |
| `sample/` | Place source images here for testing |
| `outputs/` | Converted images written here |

---

## Prerequisites

```bash
pip install Pillow
```

### Get a sample image

```bash
# Reuse the image from recipe 50 (no download needed)
cp cookbook/51_image_caption/sample/photo.jpg \
   cookbook/57_image_convert/sample/photo.jpg
```

---

## Testing — run.py (physical runner)

`run.py` invokes Pillow directly. No LLM, no SPL executor needed.

```bash
# JPEG → PNG
python cookbook/57_image_convert/run.py \
    --image cookbook/57_image_convert/sample/photo.jpg \
    --target-format png

# JPEG → WebP (quality 90)
python cookbook/57_image_convert/run.py \
    --image cookbook/57_image_convert/sample/photo.jpg \
    --target-format webp --quality 90

# JPEG → BMP
python cookbook/57_image_convert/run.py \
    --image cookbook/57_image_convert/sample/photo.jpg \
    --target-format bmp

# Custom output directory
python cookbook/57_image_convert/run.py \
    --image cookbook/57_image_convert/sample/photo.jpg \
    --target-format webp --output-dir /tmp/converted
```

---

## Testing — SPL executor (spl run)

`spl run` exercises the `.spl` workflow directly through the SPL executor.
The `convert_image` tool must be loaded from `cookbook/tools/registry.py`.

```bash
# Default params (JPEG → JPEG, quality 85)
spl-go run cookbook/57_image_convert/image_convert.spl \
    --tools cookbook/tools/registry.py

# JPEG → WebP
spl-go run cookbook/57_image_convert/image_convert.spl \
    --tools cookbook/tools/registry.py \
    -p image=cookbook/57_image_convert/sample/photo.jpg \
    -p target_format=webp \
    -p quality=90

# JPEG → PNG, custom output dir
spl-go run cookbook/57_image_convert/image_convert.spl \
    --tools cookbook/tools/registry.py \
    -p image=cookbook/57_image_convert/sample/photo.jpg \
    -p target_format=png \
    -p output_dir=/tmp/converted
```

> No `--adapter` needed — this workflow has no `GENERATE` statements.

---

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `@image` | IMAGE | `cookbook/57_image_convert/sample/photo.png` | Source image file path |
| `@target_format` | TEXT | `jpeg` | Target format: `jpeg`, `png`, `webp`, `bmp`, `gif` |
| `@quality` | INT | `85` | Compression quality for lossy formats (1–95); ignored for lossless |
| `@output_dir` | TEXT | `cookbook/57_image_convert/outputs` | Directory to write converted file |
| `@log_dir` | TEXT | `cookbook/57_image_convert/logs-spl` | Directory for log output |

---

## Output

`@converted IMAGE` — file path of the converted image in `@output_dir`.

Filename pattern: `<original_stem>.<target_format>` (e.g. `photo.webp`).

---

## Composability

`image_convert` is designed to be composed with other recipes via `CALL`:

```sql
-- Normalise input format before captioning
CALL image_convert(@raw_image, 'jpeg', 85, @output_dir) INTO @image
CALL image_caption(@image, 'caption', @model) INTO @caption
```

---

## Error handling

| Exception | Cause | Behaviour |
|-----------|-------|-----------|
| `FileNotFound` | Source image path does not exist | Returns error string with `status = 'failed'` |
| `UnsupportedFormat` | Target format not supported by Pillow | Returns error string with `status = 'failed'` |
