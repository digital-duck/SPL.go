```bash
spl-go run cookbook/06_react_agent/react_agent.spl --adapter claude_cli --model claude-sonnet-4-6 --claude-allowed-tools WebSearch country="France"    



spl-go run cookbook/06_react_agent/react_agent.spl --adapter claude_cli --model claude-sonnet-4-6 --claude-allowed-tools WebSearch --tools cookbook/06_react_agent/tools.py country="China" \
    2>&1 | tee cookbook/out/06_react_agent-$(date +%Y%m%d_%H%M%S).md 

```