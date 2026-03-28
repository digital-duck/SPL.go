package adapter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCLIAdapter runs Claude via the `claude` CLI tool.
type ClaudeCLIAdapter struct {
	CLIPath      string
	DefaultModel string
	Timeout      time.Duration
}

// NewClaudeCLIAdapter creates a new ClaudeCLIAdapter from optional config.
func NewClaudeCLIAdapter(cfg map[string]string) *ClaudeCLIAdapter {
	cliPath := "claude"
	if cfg != nil {
		if v, ok := cfg["cli_path"]; ok && v != "" {
			cliPath = v
		}
	}

	defaultModel := "claude-sonnet-4-6"
	if cfg != nil {
		if v, ok := cfg["default_model"]; ok && v != "" {
			defaultModel = v
		} else if v, ok := cfg["model"]; ok && v != "" {
			defaultModel = v
		}
	}

	return &ClaudeCLIAdapter{
		CLIPath:      cliPath,
		DefaultModel: defaultModel,
		Timeout:      300 * time.Second,
	}
}

func (a *ClaudeCLIAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if model == "" {
		model = a.DefaultModel
	}

	// Verify that the claude CLI is available
	if _, err := exec.LookPath(a.CLIPath); err != nil {
		return nil, fmt.Errorf("claude_cli: %q not found in PATH: %w", a.CLIPath, err)
	}

	// Build the full prompt (prepend system if provided)
	fullPrompt := prompt
	if system != "" {
		fullPrompt = system + "\n\n" + prompt
	}

	// Create a context with the timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, a.CLIPath,
		"-p", fullPrompt,
		"--no-session-persistence",
		"--model", model,
		"--tools", "",
	)

	// Strip sensitive env vars
	env := os.Environ()
	filtered := env[:0]
	stripKeys := map[string]bool{
		"ANTHROPIC_API_KEY":       true,
		"ANTHROPIC_BASE_URL":      true,
		"CLAUDECODE":              true,
		"CLAUDE_CODE_ENTRYPOINT":  true,
	}
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if !stripKeys[parts[0]] {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered

	start := time.Now()
	out, err := cmd.Output()
	latencyMs := float64(time.Since(start).Milliseconds())

	if err != nil {
		if timeoutCtx.Err() != nil {
			return nil, fmt.Errorf("claude_cli: timed out after %s", a.Timeout)
		}
		return nil, fmt.Errorf("claude_cli: command failed: %w", err)
	}

	content := strings.TrimSpace(string(out))
	if content == "" {
		return nil, fmt.Errorf("claude_cli: empty response from CLI")
	}

	inputTokens := len(fullPrompt) / 4
	outputTokens := len(content) / 4

	return &GenerationResult{
		Content:      content,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		LatencyMs:    latencyMs,
		CostUSD:      0.0, // subscription billing
	}, nil
}

func (a *ClaudeCLIAdapter) CountTokens(text, model string) int {
	return len(text) / 4
}

func (a *ClaudeCLIAdapter) ListModels() []string {
	return []string{"claude-sonnet-4-6"}
}

func (a *ClaudeCLIAdapter) Name() string {
	return "claude_cli"
}
