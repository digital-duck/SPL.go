package adapter

import "context"

// EchoAdapter returns the prompt as its output. Useful for testing.
type EchoAdapter struct{}

// NewEchoAdapter creates a new EchoAdapter.
func NewEchoAdapter() *EchoAdapter {
	return &EchoAdapter{}
}

func (a *EchoAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	tokens := len(prompt) / 4
	return &GenerationResult{
		Content:      prompt,
		Model:        "echo",
		InputTokens:  tokens,
		OutputTokens: tokens,
		TotalTokens:  tokens * 2,
		LatencyMs:    1.0,
		CostUSD:      0.0,
	}, nil
}

func (a *EchoAdapter) CountTokens(text, model string) int {
	return len(text) / 4
}

func (a *EchoAdapter) ListModels() []string {
	return []string{"echo"}
}

func (a *EchoAdapter) Name() string {
	return "echo"
}
