# Recipe 01: Hello World

Minimal SPL program — verifies that `spl`, the chosen adapter, and the model are all wired up correctly. This recipe supports parameters and is optimized for both large and small (local) models.


✦ To use SPL 3.0 properly in your environment, you should use the spl123 Conda environment and perform an editable
  installation. This ensures that any changes you make to the SPL source code are immediately reflected when you run the
  spl command.

1. Activate the SPL 3.0 Environment
Since you already have a spl123 environment created:

```bash
conda activate spl123
```

2. Perform an Editable Installation
Navigate to your project root and install SPL with its development dependencies:

3. Verify the Setup
Check that the spl-go command is now pointing to your local directory:

```bash
which spl-go
# ~/.local/bin/spl-go
spl-go --help
```

Pro Tip: Running without installation
If you ever want to run the code strictly from the current directory without relying on the pip path, you can use:



## Usage

```bash
# Echo adapter (no LLM needed — mirrors prompt back)
spl-go run  cookbook/01_hello_world/hello.spl

# Local Ollama (default model)
spl-go run  cookbook/01_hello_world/hello.spl --adapter ollama

# With parameters (translated greeting)
spl-go run  cookbook/01_hello_world/hello.spl --adapter ollama \
    --param user_input="hello wen" \
    --param lang="Chinese"

# Specific model
spl-go run  cookbook/01_hello_world/hello.spl --adapter ollama --model llama3.2
```

```bash
export FILE_SPL="$HOME/projects/digital-duck/SPL20/cookbook/01_hello_world/hello.spl"

## different commands
# python
spl    run $FILE_SPL --adapter ollama --model gemma3 --param user_input="hello wen" --param lang="Chinese"

# TypeScript
spl-ts run $FILE_SPL --adapter ollama --model gemma3 --param user_input="hello wen" --param lang="Chinese"

# Go
spl-go run $FILE_SPL --adapter ollama --model gemma3 --param user_input="hello wen" --param lang="Chinese"
```


## What it does

1.  **Defines a Template**: Uses `CREATE FUNCTION greeting()` to provide a structured prompt template.
2.  **Handles Context**: `SELECT`s `user_input` and `lang` from the execution context (passed via `--param`).
3.  **Generates Response**: Instructs the model to introduce itself and SPL 3.0 in two sentences, respecting the requested language and user input.

Using an explicit `CREATE FUNCTION` ensures that smaller models (like `llama3.2` or `gemma2`) provide a direct response instead of attempting to "write code" for the function.
