# Recipe 65: LLM-powered SPL Compiler (vibe-splc)

Compiles a `.spl` workflow to a target framework implementation using LLM (`claude_cli`).
Complements the deterministic `splc` compiler — run both and cross-compare to surface gaps
between generated output and the author's intent.

**Supported targets:** `crewai`, `autogen`, `langgraph`  
**Recommended adapter:** `claude_cli`  
**Default model in generated code:** `gemma3` (Ollama-compatible)

## How it works

1. Loads the `.spl` source file
2. Generates an idiomatic target framework implementation via LLM
3. Self-reviews the output against the original SPL spec
4. Fixes any gaps found before writing the final file

## Fill the §6.2 table gaps

### Recipe 11: debate_arena → CrewAI

```bash
spl-go run cookbook/65_llm_splc/llm_splc.spl \
    --adapter claude_cli \
    --tools cookbook/65_llm_splc/tools.py \
    --param spl_file="$(pwd)/cookbook/11_debate_arena/debate.spl" \
    --param target="crewai" \
    --param output_file="$(pwd)/cookbook/65_llm_splc/targets/crewai/debate_arena.py"
```

### Recipe 63: parallel_code_review → AutoGen

```bash
spl-go run cookbook/65_llm_splc/llm_splc.spl \
    --adapter claude_cli \
    --tools cookbook/65_llm_splc/tools.py \
    --param spl_file="$(pwd)/cookbook/63_parallel_code_review/parallel_code_review.spl" \
    --param target="autogen" \
    --param output_file="$(pwd)/cookbook/65_llm_splc/targets/autogen/parallel_code_review.py"
```

### Recipe 63: parallel_code_review → CrewAI

```bash
spl-go run cookbook/65_llm_splc/llm_splc.spl \
    --adapter claude_cli \
    --tools cookbook/65_llm_splc/tools.py \
    --param spl_file="$(pwd)/cookbook/63_parallel_code_review/parallel_code_review.spl" \
    --param target="crewai" \
    --param output_file="$(pwd)/cookbook/65_llm_splc/targets/crewai/parallel_code_review.py"
```

```bash
spl-go run cookbook/65_llm_splc/llm_splc.spl \
    --adapter ollama --model=gemma3 \
    --tools cookbook/65_llm_splc/tools.py \
    --param spl_file="$(pwd)/cookbook/63_parallel_code_review/parallel_code_review.spl" \
    --param target="crewai" \
    --param output_file="$(pwd)/cookbook/65_llm_splc/targets-ollama/crewai/parallel_code_review.py"
```


## General usage

```bash
spl-go run cookbook/65_llm_splc/llm_splc.spl \
    --adapter claude_cli \
    --tools cookbook/65_llm_splc/tools.py \
    --param spl_file="$(pwd)/<path/to/workflow.spl>" \
    --param target="<crewai|autogen|langgraph>" \
    --param output_file="$(pwd)/<path/to/output.py>"
```

## Cross-validation with deterministic splc

```bash
# Deterministic compile
spl-go splc cookbook/05_self_refine/self_refine.spl \
    --lang python/langgraph \
    --out-dir cookbook/05_self_refine/targets/python_langgraph

# LLM compile
spl-go run cookbook/65_llm_splc/llm_splc.spl \
    --adapter claude_cli \
    --tools cookbook/65_llm_splc/tools.py \
    --param spl_file="$(pwd)/cookbook/05_self_refine/self_refine.spl" \
    --param target="langgraph" \
    --param output_file="$(pwd)/cookbook/65_llm_splc/targets/langgraph/self_refine.py"

# Diff
diff cookbook/05_self_refine/targets/python_langgraph/self_refine_python_langgraph.py \
     cookbook/65_llm_splc/targets/langgraph/self_refine.py
```

Gaps in the diff surface implicit logic the author embedded procedurally —
making the declarative intent explicit and auditable.
