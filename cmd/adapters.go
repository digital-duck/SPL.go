package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var adaptersCmd = &cobra.Command{
	Use:   "adapters",
	Short: "List available LLM adapters",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available SPL 2.0 adapters:")
		fmt.Println()
		fmt.Printf("  %-12s  %s\n", "Adapter", "Description")
		fmt.Printf("  %-12s  %s\n", "-------", "-----------")
		fmt.Printf("  %-12s  %s\n", "echo",     "Echo adapter — returns prompt as output (testing/dry-run)")
		fmt.Printf("  %-12s  %s\n", "ollama",   "Ollama adapter — local LLM via Ollama server (default: localhost:11434)")
		fmt.Printf("  %-12s  %s\n", "momagrid",   "Momagrid adapter — distributed LLM grid (default: localhost:9000)")
		fmt.Printf("  %-12s  %s\n", "anthropic",  "Anthropic Claude API (requires ANTHROPIC_API_KEY)")
		fmt.Printf("  %-12s  %s\n", "claude_cli", "Claude Code CLI — subscription billing, $0 per call")
		fmt.Printf("  %-12s  %s\n", "openrouter", "OpenRouter — access 100+ models via one API (requires OPENROUTER_API_KEY)")
		fmt.Printf("  %-12s  %s\n", "openai",     "OpenAI API — GPT-4o, o1, o3 models (requires OPENAI_API_KEY)")
		fmt.Printf("  %-12s  %s\n", "deepseek",   "DeepSeek API — deepseek-chat, deepseek-reasoner (requires DEEPSEEK_API_KEY)")
		fmt.Printf("  %-12s  %s\n", "qwen",       "Alibaba Cloud Qwen API — qwen-plus, qwen-max, qwen2.5-* (requires DASHSCOPE_API_KEY)")
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  ~/.spl/config.yaml           — global adapter config")
		fmt.Println("  OLLAMA_BASE_URL               — override Ollama server URL")
		fmt.Println("  MOMAGRID_HUB_URL              — override Momagrid hub URL")
		fmt.Println("  MOMAGRID_API_KEY              — Momagrid authentication key")
		fmt.Println("  ANTHROPIC_API_KEY             — Anthropic API key")
		fmt.Println("  OPENROUTER_API_KEY            — OpenRouter API key")
		fmt.Println("  OPENAI_API_KEY                — OpenAI API key")
		fmt.Println("  DEEPSEEK_API_KEY              — DeepSeek API key")
		fmt.Println("  DASHSCOPE_API_KEY             — Alibaba Cloud DashScope API key (Qwen)")
	},
}
