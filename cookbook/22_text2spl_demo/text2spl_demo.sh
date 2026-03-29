#!/usr/bin/env bash
# Recipe 22: text2SPL Compiler Demo (spl-go / spl-go native)
#
# Showcases the natural language → SPL 2.0 compiler built into spl-go.
# No Python, no venv — just the spl-go binary aliased as spl.
#
# Usage:
#   bash 22_text2spl_demo/text2spl_demo.sh [adapter] [model]
#
# Examples:
#   bash 22_text2spl_demo/text2spl_demo.sh ollama gemma3
#   bash 22_text2spl_demo/text2spl_demo.sh claude_cli
#   bash 22_text2spl_demo/text2spl_demo.sh anthropic claude-sonnet-4-6

set -uo pipefail

ADAPTER="${1:-ollama}"
MODEL="${2:-gemma3}"
OUTDIR="22_text2spl_demo/generated"
mkdir -p "$OUTDIR"

PASS=0
FAIL=0

run_demo() {
    local num="$1" label="$2" desc="$3" mode="$4" outfile="$5"

    echo "--- Demo $num: $label ---"
    echo "  Input:  '$desc'"
    echo "  Mode:   $mode"
    echo ""

    if spl text2spl "$desc" \
        --adapter "$ADAPTER" -m "$MODEL" --mode "$mode" --no-validate -o "$outfile" 2>&1; then
        echo ""
        echo "  Validating generated code..."
        if spl validate "$outfile" 2>&1; then
            echo "  [validation: OK]"
        else
            echo "  [validation: warning — generated code has issues (known limitation for $mode mode)]"
        fi
        PASS=$((PASS + 1))
    else
        echo "  [generation: FAILED]"
        FAIL=$((FAIL + 1))
    fi
    echo ""
}

echo "=== SPL 2.0 text2SPL Compiler Demo (spl-go) ==="
echo "    Adapter: $ADAPTER  Model: $MODEL"
echo "    Runtime: $(spl version)"
echo ""

run_demo 1 "Compile a simple prompt" \
    "summarize a document with a 2000 token budget" \
    "prompt" "$OUTDIR/summarize.spl"

run_demo 2 "Compile a multi-step workflow" \
    "build a review agent that drafts, critiques, and refines text until quality > 0.8" \
    "workflow" "$OUTDIR/review_agent.spl"

run_demo 3 "Auto mode — LLM decides the best form" \
    "classify user intent and route to the right handler" \
    "auto" "$OUTDIR/classifier.spl"

echo "=== Generated files ==="
ls -la "$OUTDIR"/*.spl 2>/dev/null || echo "  (no files generated)"
echo ""
echo "=== Demo complete: $PASS passed, $FAIL failed ==="
echo "  To view:    cat $OUTDIR/summarize.spl"
echo "  To execute: spl run $OUTDIR/summarize.spl --adapter $ADAPTER"

if [ "$PASS" -eq 0 ] && [ "$FAIL" -gt 0 ]; then
    exit 1
fi
