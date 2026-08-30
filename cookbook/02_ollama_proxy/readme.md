# Recipe 02: Ollama Proxy

General-purpose LLM query — proxy any prompt to any adapter/model. This recipe is a "universal adapter" that lets you interact with any LLM using standard SPL parameters.

## Usage

```bash
export FILE_SPL="$HOME/projects/digital-duck/SPL.py/cookbook/02_ollama_proxy/proxy.spl"

## different commands
# python
spl-go   run $FILE_SPL --adapter ollama --model gemma3 --param prompt="What is 10! ?"

# TypeScript
spl-ts run $FILE_SPL --adapter ollama --model gemma3 --param prompt="What is 10! ?"

# Go
spl-go run $FILE_SPL --adapter ollama --model gemma3 --param prompt="What is 10! ?"




# General usage
spl-go run $FILE_SPL --adapter ollama --model gemma3 --param prompt="What is 10! ?"

# Short form (trailing argument)
spl-go run $FILE_SPL --adapter ollama --model llama3.2 prompt="Write a haiku about coding"

# Different adapter
spl-go run $FILE_SPL --adapter anthropic prompt="Summarise the history of AI"

```

## Parameters

- `prompt` (required): The question or instruction for the LLM.

## What it does

1.  **Defines a Proxy Function**: `CREATE FUNCTION answer(prompt TEXT)` acts as a simple pass-through.
2.  **Binds Context**: `SELECT context.prompt AS prompt` captures the user input.
3.  **Executes**: `GENERATE answer(prompt)` sends the prompt to the model.

This explicit structure ensures compatibility across all SPL implementations (Python, TypeScript, etc.) and provides a clear audit trail in the logs.
