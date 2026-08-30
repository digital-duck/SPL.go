# Recipe 80: OpenCV 5

**Category:** vision / hybrid deterministic-tool + LLM reasoning
**SPL version:** 3.0 (`CALL`, `CALL PARALLEL`, `TOOL_API`)
**LLM required:** yes, for the reasoning step only — a small local model via Ollama is enough (tested with `gemma4`)
**Vision required:** [OpenCV 5](https://opencv.org/opencv-5/) (`opencv-python>=5.0.0.93`) — no LLM vision model, no downloaded weights, fully offline
**Demonstrates:** OpenCV as a deterministic sensing layer feeding SPL's `GENERATE` reasoning; `CALL PARALLEL` fanning out across concurrent tool calls (not just concurrent LLM calls)

Two recipes showing OpenCV 5 as SPL's vision "sense" layer: OpenCV measures pixels deterministically and cheaply, SPL's LLM step reasons over the resulting structured report — never over raw pixels. Same principle as `36_tool_use` ("LLMs are great at language, terrible at arithmetic — keep them in their lane"), applied to vision instead of arithmetic.

---

## What's new in OpenCV 5 (released June 2026)

OpenCV 5.0 launched at CVPR 2026 (Denver, June 4) with the biggest architecture change in years:

| Area | What changed |
|---|---|
| **DNN engine** | Complete rewrite to a typed operation graph (shape inference, constant folding, operator fusion). Three engines coexist behind `cv::dnn::EngineType`: `ENGINE_CLASSIC` (4.x-style), `ENGINE_NEW` (graph-based), `ENGINE_ORT` (bundled ONNX Runtime), `ENGINE_AUTO` |
| **ONNX coverage** | ~22% (4.x) → **>80%** (5.0), including `If`/`Loop` subgraphs and transformer-style attention fusion |
| **Native LLM/VLM support** | The DNN module now ships a tokenizer + KV-cache for autoregressive decoding — Qwen 2.5, Gemma 3, PaliGemma, and GPT-family models can run "image in, text out" directly through OpenCV, with no external LLM framework |
| **New data types** | `cv::hfloat` / `CV_16F` (fp16) and `cv::bfloat` / `CV_16BF` (bf16) are first-class Mat depths, alongside bool and 64-bit int |
| **Hardware Acceleration Layer (HAL)** | Automatic dispatch to vendor kernels: Intel IPP (x86/x64), Arm KleidiCV (AArch64 — the architecture most robot/edge compute runs on), Qualcomm FastCV (Snapdragon/Hexagon DSP), RISC-V Vector |
| **3D vision reorg** | `calib3d` split into `3d` (geometry/ICP/SLAM), `calib` (calibration incl. hand-eye/robot-world), `stereo` (depth) — plus TSDF/HashTSDF volumes and dense RGB-D fusion |
| **Python API** | Keyword arguments now work on core APIs (previously positional-only); NumPy 2.x support |
| **Requirements** | C++17 minimum; Python 2 bindings removed entirely |

Sources: [opencv.org/opencv-5](https://opencv.org/opencv-5/), [Phoronix](https://www.phoronix.com/news/OpenCV-5.0-Released), [OpenCV 5 wiki](https://github.com/opencv/opencv/wiki/OpenCV-5)

**Why the HAL and 3D-vision items matter beyond a changelog:** KleidiCV/FastCV backends and the SLAM-oriented `3d`/`calib` split are squarely aimed at running vision on embedded/edge/robot hardware, not desktop GPUs — i.e. OpenCV 5 is explicitly positioning itself as a physical-AI / embodied-AI vision layer, which is exactly the sensing substrate an AI-orchestrated robot needs.

### What this recipe actually exercises (verified against `opencv-python==5.0.0.93`)

- `CV_16F` (fp16) `Mat`s in `detect_change` — confirmed `cv2.absdiff` works directly on `float16` arrays.
- Keyword arguments on core APIs (`cv2.HoughCircles(image=..., method=...)`, `cv2.calcHist(images=...)`, `cv2.Canny(image=...)`) — confirmed working; positional-only in 4.x.
- **Not used**: `CV_16BF` (bf16) is documented for 5.0 but **not yet exposed in the Python bindings** of this wheel (`hasattr(cv2, 'CV_16BF')` is `False` as of `5.0.0.93`) — mentioned above for completeness, not relied on in code.
- **Not used**: bundled Haar cascade XML files are **not shipped** in this `opencv-python` wheel (`cv2.data.haarcascades` resolves to an empty directory) — this recipe uses `cv2.HoughCircles` (core `imgproc`, always available, no external data file) instead of face/object cascade detection for that reason.

---

## Getting started

```bash
conda activate spl123

# 1. Install OpenCV 5 (and Pillow, only needed to regenerate sample images)
pip install -r cookbook/80_opencv_5/requirements.txt

# 2. Verify the install and the specific 5.0 features this recipe uses
python -c "
import cv2
print('cv2 version:', cv2.__version__)
print('CV_16F available:', hasattr(cv2, 'CV_16F'))
"
# Expect: cv2 version: 5.0.0 / CV_16F available: True

# 3. Make sure Ollama is running with a small model pulled
ollama pull gemma4
```

### Regenerate the sample test images (optional — already committed under `sample/`)

```bash
python3 - << 'EOF'
from PIL import Image, ImageDraw
import pathlib

out = pathlib.Path("cookbook/80_opencv_5/sample")
out.mkdir(parents=True, exist_ok=True)

# background.jpg — static reference scene
bg = Image.new("RGB", (400, 300), (135, 206, 235))
d = ImageDraw.Draw(bg)
d.polygon([(0, 300), (150, 120), (300, 300)], fill=(90, 90, 90))
d.polygon([(120, 300), (280, 90), (400, 300)], fill=(110, 110, 110))
bg.save(out / "background.jpg", "JPEG", quality=90)

# current.jpg — same scene, one new high-contrast object appeared
cur = bg.copy()
d2 = ImageDraw.Draw(cur)
d2.ellipse([250, 30, 330, 110], fill=(20, 20, 20))
cur.save(out / "current.jpg", "JPEG", quality=90)
EOF
```

> Use a genuinely high-contrast color for the added object. An earlier version of this fixture used a yellow circle on a sky-blue background — both convert to nearly the same grayscale value (~204 vs ~188), so the `CV_16F` diff barely crossed the detection threshold. Dark-on-light (or vice versa) is a much more reliable test signal.

---

## Recipe A: `opencv5_scene_triage.spl` — deterministic tool + LLM reasoning

Compares a background reference frame against a current frame, reports what changed, then has the LLM turn the structured report into a plain-language assessment.

```bash
spl-go run cookbook/80_opencv_5/opencv5_scene_triage.spl \
    --llm ollama:gemma4 \
    --param background=cookbook/80_opencv_5/sample/background.jpg \
    --param current=cookbook/80_opencv_5/sample/current.jpg
```

**Pipeline:**
```
detect_change(background, current)   ← TOOL_API, OpenCV 5, CV_16F diff, 0 LLM calls
        │  JSON: {changed_pixel_ratio, num_regions, largest_region_bbox, motion_detected}
        ▼
GENERATE scene_assessment_prompt(change_report)   ← 1 LLM call, reasons over JSON only
        │
        ▼
@assessment TEXT
```

Verified output on the committed sample images:
```json
{"changed_pixel_ratio": 0.0429, "num_regions": 1, "largest_region_bbox": [250, 30, 81, 81], "motion_detected": true}
```
> "Motion was detected due to a change covering approximately 4.3% of the total frame area. The motion is highly localized, contained within a single small region rather than affecting the entire scene..."

---

## Recipe B: `opencv5_parallel_inspect.spl` — `CALL PARALLEL` over deterministic tools

`CALL PARALLEL` is usually shown fanning out across concurrent *LLM* calls (see `64_parallel_news_digest`). This recipe fans it out across three independent, deterministic OpenCV 5 analyses on the *same* image instead, then synthesizes all three with a single `GENERATE` call.

```bash
spl-go run cookbook/80_opencv_5/opencv5_parallel_inspect.spl \
    --llm ollama:gemma4 \
    --param image=cookbook/80_opencv_5/sample/current.jpg
```

**Pipeline:**
```
                 ┌── analyze_circles(image)  → circle count + positions (Hough transform)
CALL PARALLEL ───┼── analyze_exposure(image) → brightness / under-/over-exposure ratios
                 └── analyze_edges(image)    → edge-pixel density (Canny)
                         │  all three: 0 LLM calls, ran concurrently
                         ▼
GENERATE inspection_synthesis_prompt(circles, exposure, edges)   ← 1 LLM call
                         ▼
                 @note TEXT
```

Verified run: all three branches completed in 1–30ms each (0 LLM calls), followed by one ~13s LLM synthesis call producing a coherent 4-sentence inspection note referencing the detected circle's actual position and the measured exposure/edge numbers.

**Gotcha worth knowing if you write your own `CALL PARALLEL` recipe:** each branch must be a top-level `WORKFLOW` (registered in the composer's registry) — a bare `TOOL_API` call or a `PROCEDURE` in a parallel branch fails with `Unknown workflow '<name>'`. That's why `analyze_circles`/`analyze_exposure`/`analyze_edges` are thin `WORKFLOW` wrappers around the actual `TOOL_API` calls, not `PROCEDURE`s or direct tool calls — confirmed by testing both alternatives against this SPL build before landing on the working pattern.

---

## Files

| File | Role |
|---|---|
| `opencv5_scene_triage.spl` | Sequential hybrid pattern: 1 deterministic tool → 1 GENERATE |
| `opencv5_parallel_inspect.spl` | `CALL PARALLEL` fan-out over 3 deterministic tools → 1 GENERATE |
| `tools.spl` | `detect_change`, `detect_circular_objects`, `measure_exposure`, `detect_edge_density` — all pure OpenCV 5, zero LLM cost |
| `requirements.txt` | `opencv-python>=5.0.0.93`, `numpy`, `Pillow` (fixture generation only) |
| `sample/background.jpg`, `sample/current.jpg` | Synthetic test fixtures (procedurally generated, not real photos) |

---

## Composability

Both tools return plain JSON strings, so they compose with any other SPL recipe the same way `28_support_triage` or `48_credit_risk` do:

```sql
-- Feed a detected region into the existing image-captioning recipe for a
-- richer, LLM-authored description of *why* something changed, not just that it did
CALL detect_change(@background, @current) INTO @change_report
CALL image_caption(@current, 'Describe what changed in this frame', 'caption', @model, 512) INTO @caption
```

---

## Error handling

| Exception | Cause | Behaviour |
|---|---|---|
| `GenerationError` | Ollama not running or model unavailable | Returns the raw OpenCV JSON report with `status = 'report_only'` / `'partial'` instead of failing — the deterministic signal survives even if the LLM doesn't |
