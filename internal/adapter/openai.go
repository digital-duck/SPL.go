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

const openAIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIAdapter routes requests through the OpenAI API.
// Supports GPT-4o, GPT-4, GPT-3.5, o1, o3 models.
// API key: OPENAI_API_KEY env var or config "api_key".
type OpenAIAdapter struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Timeout      time.Duration
	client       *http.Client
}

// NewOpenAIAdapter creates an OpenAIAdapter from optional config.
func NewOpenAIAdapter(cfg map[string]string) *OpenAIAdapter {
	baseURL := openAIURL
	defaultModel := "gpt-4o"
	apiKey := os.Getenv("OPENAI_API_KEY")
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

	return &OpenAIAdapter{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		Timeout:      timeout,
		client:       &http.Client{Timeout: timeout},
	}
}

func (a *OpenAIAdapter) Name() string { return "openai" }

func (a *OpenAIAdapter) ListModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1",
		"o1-mini",
		"o3",
		"o3-mini",
	}
}

func (a *OpenAIAdapter) CountTokens(text, model string) int {
	if text == "" {
		return 0
	}
	// GPT family: ~4 chars per token
	return len(text) / 4
}

func (a *OpenAIAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("openai: API key required — set OPENAI_API_KEY or config api_key")
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
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := a.client.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("openai: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: API error %d: %s", resp.StatusCode, string(raw))
	}

	data, err := parseOpenRouterResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	choices, _ := data["choices"].([]interface{})
	content := ""
	if len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				content, _ = msg["content"].(string)
				// Reasoning model fallback: o1/o3 may return empty content
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

	costUSD := estimateOpenAICost(model, inputTokens, outputTokens)

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

// estimateOpenAICost returns approximate USD cost based on model and token counts.
func estimateOpenAICost(model string, inputTokens, outputTokens int) float64 {
	m := strings.ToLower(model)

	var inputPer1M, outputPer1M float64
	switch {
	case strings.HasPrefix(m, "gpt-4o-mini"):
		inputPer1M, outputPer1M = 0.15, 0.60
	case strings.HasPrefix(m, "gpt-4o"):
		inputPer1M, outputPer1M = 2.50, 10.0
	case strings.HasPrefix(m, "gpt-4-turbo"):
		inputPer1M, outputPer1M = 10.0, 30.0
	case strings.HasPrefix(m, "gpt-4"):
		inputPer1M, outputPer1M = 30.0, 60.0
	case strings.HasPrefix(m, "gpt-3.5"):
		inputPer1M, outputPer1M = 0.50, 1.50
	case strings.HasPrefix(m, "o1"):
		inputPer1M, outputPer1M = 15.0, 60.0
	case strings.HasPrefix(m, "o3"):
		inputPer1M, outputPer1M = 10.0, 40.0
	default:
		inputPer1M, outputPer1M = 2.50, 10.0
	}

	return (float64(inputTokens)*inputPer1M + float64(outputTokens)*outputPer1M) / 1_000_000
}
