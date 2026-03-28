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

const qwenURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions"

// QwenAdapter routes requests through the Alibaba Cloud Qwen API.
// Uses OpenAI-compatible /chat/completions protocol (DashScope international endpoint).
// API key: DASHSCOPE_API_KEY env var or config "api_key".
type QwenAdapter struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Timeout      time.Duration
	client       *http.Client
}

// NewQwenAdapter creates a QwenAdapter from optional config.
func NewQwenAdapter(cfg map[string]string) *QwenAdapter {
	baseURL := qwenURL
	defaultModel := "qwen-plus"
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
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

	return &QwenAdapter{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		Timeout:      timeout,
		client:       &http.Client{Timeout: timeout},
	}
}

func (a *QwenAdapter) Name() string { return "qwen" }

func (a *QwenAdapter) ListModels() []string {
	return []string{
		"qwen-max",
		"qwen-plus",
		"qwen-turbo",
		"qwen-long",
		"qwen2.5-72b-instruct",
		"qwen2.5-32b-instruct",
		"qwen2.5-14b-instruct",
		"qwen2.5-7b-instruct",
		"qwen2.5-coder-32b-instruct",
	}
}

func (a *QwenAdapter) CountTokens(text, model string) int {
	if text == "" {
		return 0
	}
	// Qwen: ~3.5 chars per token
	return len(text) / 4
}

func (a *QwenAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("qwen: API key required — set DASHSCOPE_API_KEY or config api_key")
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
		return nil, fmt.Errorf("qwen: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("qwen: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := a.client.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("qwen: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qwen: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qwen: API error %d: %s", resp.StatusCode, string(raw))
	}

	data, err := parseOpenRouterResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("qwen: %w", err)
	}

	choices, _ := data["choices"].([]interface{})
	content := ""
	if len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				content, _ = msg["content"].(string)
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

	costUSD := estimateQwenCost(model, inputTokens, outputTokens)

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

// estimateQwenCost returns approximate USD cost based on model and token counts.
func estimateQwenCost(model string, inputTokens, outputTokens int) float64 {
	m := strings.ToLower(model)

	var inputPer1M, outputPer1M float64
	switch {
	case strings.Contains(m, "qwen-max"):
		inputPer1M, outputPer1M = 1.6, 6.4
	case strings.Contains(m, "qwen-turbo"):
		inputPer1M, outputPer1M = 0.06, 0.20
	default: // qwen-plus, qwen-long, qwen2.5-*
		inputPer1M, outputPer1M = 0.4, 1.2
	}

	return (float64(inputTokens)*inputPer1M + float64(outputTokens)*outputPer1M) / 1_000_000
}
