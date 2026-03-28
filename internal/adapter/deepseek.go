package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const deepSeekURL = "https://api.deepseek.com/v1/chat/completions"

// DeepSeekAdapter routes requests through the DeepSeek API.
// Uses OpenAI-compatible /chat/completions protocol.
// API key: DEEPSEEK_API_KEY env var or config "api_key".
type DeepSeekAdapter struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Timeout      time.Duration
	client       *http.Client
}

// NewDeepSeekAdapter creates a DeepSeekAdapter from optional config.
func NewDeepSeekAdapter(cfg map[string]string) *DeepSeekAdapter {
	baseURL := deepSeekURL
	defaultModel := "deepseek-chat"
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	timeout := 180 * time.Second

	if cfg != nil {
		if v, ok := cfg["base_url"]; ok && v != "" {
			baseURL = v
		}
		if v, ok := cfg["default_model"]; ok && v != "" {
			defaultModel = v
		}
		if v, ok := cfg["api_key"]; ok && v != "" {
			apiKey = v
		}
	}

	return &DeepSeekAdapter{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		Timeout:      timeout,
		client:       &http.Client{Timeout: timeout},
	}
}

func (a *DeepSeekAdapter) Name() string { return "deepseek" }

func (a *DeepSeekAdapter) ListModels() []string {
	return []string{
		"deepseek-chat",
		"deepseek-reasoner",
	}
}

func (a *DeepSeekAdapter) CountTokens(text, model string) int {
	if text == "" {
		return 0
	}
	// DeepSeek: ~3.5 chars per token
	return len(text) / 4
}

func (a *DeepSeekAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("deepseek: API key required — set DEEPSEEK_API_KEY or config api_key")
	}
	if model == "" {
		model = a.DefaultModel
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := []message{}
	if system != "" {
		messages = append(messages, message{Role: "system", Content: system})
	}
	messages = append(messages, message{Role: "user", Content: prompt})

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("deepseek: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("deepseek: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := a.client.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("deepseek: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deepseek: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deepseek: API error %d: %s", resp.StatusCode, string(raw))
	}

	data, err := parseOpenRouterResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("deepseek: %w", err)
	}

	choices, _ := data["choices"].([]interface{})
	content := ""
	if len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				content, _ = msg["content"].(string)
				// Reasoning model fallback: deepseek-reasoner returns reasoning_content
				if strings.TrimSpace(content) == "" {
					if r, ok := msg["reasoning_content"].(string); ok && r != "" {
						content = r
					} else if r, ok := msg["reasoning"].(string); ok && r != "" {
						content = r
					}
				}
			}
		}
	}

	inputTokens, outputTokens := 0, 0
	if usage, ok := data["usage"].(map[string]interface{}); ok {
		if v, ok := usage["prompt_tokens"].(float64); ok {
			inputTokens = int(v)
		}
		if v, ok := usage["completion_tokens"].(float64); ok {
			outputTokens = int(v)
		}
	}

	costUSD := estimateDeepSeekCost(model, inputTokens, outputTokens)

	return &GenerationResult{
		Content:      content,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		LatencyMs:    latencyMs,
		CostUSD:      costUSD,
	}, nil
}

// estimateDeepSeekCost returns approximate USD cost based on model and token counts.
func estimateDeepSeekCost(model string, inputTokens, outputTokens int) float64 {
	m := strings.ToLower(model)

	var inputPer1M, outputPer1M float64
	switch {
	case strings.Contains(m, "deepseek-reasoner"):
		inputPer1M, outputPer1M = 0.55, 2.19
	default: // deepseek-chat and others
		inputPer1M, outputPer1M = 0.27, 1.10
	}

	return (float64(inputTokens)*inputPer1M + float64(outputTokens)*outputPer1M) / 1_000_000
}
