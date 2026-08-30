# Recipe 03: Multilingual Greeting

Greet in any language — demonstrates parametric context passing via `user_input` and `lang`.

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `user_input` | TEXT | *(required)* | The greeting or message to translate |
| `lang` | TEXT | *(required)* | Target language (e.g. Chinese, French, Japanese, Spanish) |

## Usage

```bash
spl-go run cookbook/03_multilingual/multilingual.spl --adapter ollama --model phi4 \
    --param user_input="What is Python"


```
