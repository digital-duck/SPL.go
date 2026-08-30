Run it:
```bash
spl-go run cookbook/36_tool_use/tool_use.spl \
    --adapter ollama \
    --tools cookbook/36_tool_use/tools.py \
    sales="1200,1450,1380,1600,1750,1900" \
    prev_total="7800" \
    period="H1 2025"
```

The recipe makes the architectural principle explicit in the comments:
▎ LLMs are great at language, terrible at arithmetic. Keep them in their lane.

6 CALL steps = 0 LLM calls. 1 GENERATE step = 1 LLM call for the narrative. That's the pattern.





```bash

pip install -e . 
spl-go run cookbook/06_react_agent/react_agent.spl \
    --adapter claude_cli --model claude-sonnet-4-6 \
    --claude-allowed-tools WebSearch \
    --tools cookbook/06_react_agent/tools.py \
    --timeout 200 \
    country="China"


```