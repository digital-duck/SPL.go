# Recipe 46 — MCP Integration

Discover and invoke tools via the Model Context Protocol. The LLM receives the tool list, selects the best tool for the user's question, extracts the required parameters, and dispatches either to an HTTP MCP server or a local fallback implementation.

**Category:** D.5 Cloud and Infrastructure  
**SPL constructs:** `CREATE TOOL_API`, `EVALUATE`, `CALL`, `GENERATE`  
**Difficulty:** ★☆☆

## Run

```bash
conda activate spl123
cd ~/projects/digital-duck/SPL.py

# Local-only mode (no MCP server required)
spl-go run cookbook/46_mcp/mcp.spl \
    --adapter ollama -m gemma3 \
    --param question="what is 7 factorial"

# With an MCP server
spl-go run cookbook/46_mcp/mcp.spl \
    --adapter ollama -m gemma3 \
    --param question="what is sqrt(144)" \
    --param mcp_mode=true \
    --param mcp_server_url=http://localhost:8000
```

## Pattern

```
discover_mcp_tools(url)  →  tool_list (JSON)
LLM: select_tool_and_params(question, tool_list)  →  {tool, params}
EVALUATE mcp_mode
  true  → invoke_mcp_tool(tool, params, url)   -- HTTP MCP call
  false → invoke_local_tool(tool, params)       -- local math fallback
format_final_answer(question, result)  →  answer
```

The `EVALUATE` branch on `@mcp_mode` is the key construct: the same workflow runs locally (zero dependencies) or against a live MCP server with a single parameter change — DODA in action.

## Source

Migrated from the PocketFlow open-source cookbook (`pocketflow-mcp`) using the `text2spl` pipeline.
