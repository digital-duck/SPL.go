// Package adapter defines the LLM adapter interface and factory.
package adapter

import (
	"context"
	"fmt"
)

// ContentBlock represents a single part of a multimodal message.
type ContentBlock struct {
	Type      string // "text", "image", "audio", "video"
	Text      string // for Type == "text"
	Data      []byte // for Type == "image", "audio", "video"
	MediaType string // e.g., "image/jpeg", "audio/wav"
}

// GenerationResult holds the result of a single LLM generation call.
type GenerationResult struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	LatencyMs    float64
	CostUSD      float64
}

// Adapter is the interface that all LLM backends implement.
type Adapter interface {
	// Generate sends a prompt to the LLM and returns the response.
	Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error)
	// CountTokens returns an approximate token count for the text.
	CountTokens(text, model string) int
	// ListModels returns available model names.
	ListModels() []string
	// Name returns the adapter name.
	Name() string
}

// MultimodalAdapter extends Adapter with multimodal capabilities.
type MultimodalAdapter interface {
	Adapter
	// GenerateMultimodal sends a list of content blocks to the LLM.
	GenerateMultimodal(ctx context.Context, blocks []ContentBlock, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error)
}

// New creates a new Adapter by name with optional configuration.
func New(name string, cfg map[string]string) (Adapter, error) {
	switch name {
	case "echo":
		return NewEchoAdapter(), nil
	case "ollama":
		return NewOllamaAdapter(cfg), nil
	case "momagrid":
		return NewMomagridAdapter(cfg), nil
	case "anthropic":
		return NewAnthropicAdapter(cfg), nil
	case "claude_cli":
		return NewClaudeCLIAdapter(cfg), nil
	case "openrouter":
		return NewOpenRouterAdapter(cfg), nil
	case "openai":
		return NewOpenAIAdapter(cfg), nil
	case "google":
		return NewGoogleAdapter(cfg), nil
	case "deepseek":
		return NewDeepSeekAdapter(cfg), nil
	case "qwen":
		return NewQwenAdapter(cfg), nil
	default:
		return nil, fmt.Errorf("unknown adapter: %q (available: echo, ollama, momagrid, anthropic, claude_cli, openrouter, openai, deepseek, qwen)", name)
	}
}
