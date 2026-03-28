package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterAdapter routes requests through openrouter.ai.
// Supports any model available on OpenRouter (Claude, GPT-4o, Gemini, Llama, DeepSeek, …).
// API key: OPENROUTER_API_KEY env var or config "api_key".
type OpenRouterAdapter struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Timeout      time.Duration
	client       *http.Client
}

// NewOpenRouterAdapter creates an OpenRouterAdapter from optional config.
func NewOpenRouterAdapter(cfg map[string]string) *OpenRouterAdapter {
	baseURL := openRouterURL
	defaultModel := "anthropic/claude-sonnet-4-5"
	apiKey := os.Getenv("OPENROUTER_API_KEY")
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

	return &OpenRouterAdapter{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		Timeout:      timeout,
		client:       &http.Client{Timeout: timeout},
	}
}

func (a *OpenRouterAdapter) Name() string { return "openrouter" }

func (a *OpenRouterAdapter) ListModels() []string {
	return []string{
		"anthropic/claude-sonnet-4-5",
		"anthropic/claude-haiku-3.5",
		"anthropic/claude-opus-4",
		"openai/gpt-4o",
		"openai/gpt-4o-mini",
		"google/gemini-2.0-flash-001",
		"google/gemini-2.5-pro-preview",
		"meta-llama/llama-3.3-70b-instruct",
		"deepseek/deepseek-chat-v3-0324",
		"mistralai/mistral-large",
	}
}

func (a *OpenRouterAdapter) CountTokens(text, model string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

func (a *OpenRouterAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("openrouter: API key required — set OPENROUTER_API_KEY or config api_key")
	}
	if model == "" {
		model = a.DefaultModel
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// Build messages
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
		return nil, fmt.Errorf("openrouter: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openrouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/digital-duck/SPL")
	req.Header.Set("X-Title", "SPL Engine")

	start := time.Now()
	resp, err := a.client.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("openrouter: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openrouter: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter: API error %d: %s", resp.StatusCode, string(raw))
	}

	data, err := parseOpenRouterResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}

	choices, _ := data["choices"].([]interface{})
	content := ""
	if len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				content, _ = msg["content"].(string)
				// Reasoning model fallback: some models return empty content
				// but populate reasoning or reasoning_content fields.
				if strings.TrimSpace(content) == "" {
					if r, ok := msg["reasoning"].(string); ok && r != "" {
						content = r
					} else if r, ok := msg["reasoning_content"].(string); ok && r != "" {
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

	costUSD := estimateOpenRouterCost(model, inputTokens, outputTokens)

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

// parseOpenRouterResponse attempts a 3-pass JSON parse mirroring the Python adapter:
// Pass 1: standard JSON unmarshal.
// Pass 2: strip ASCII control chars and retry.
// Pass 3: regex extraction of the content field from a truncated/malformed response.
func parseOpenRouterResponse(raw []byte) (map[string]interface{}, error) {
	// Pass 1
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err == nil {
		return data, nil
	}

	// Pass 2: strip ASCII control chars (0x00–0x1F except \t \n \r)
	cleaned := regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`).ReplaceAll(raw, nil)
	if err := json.Unmarshal(cleaned, &data); err == nil {
		return data, nil
	}

	// Pass 3: regex extraction
	re := regexp.MustCompile(`"content"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	m := re.FindSubmatch(cleaned)
	if m != nil {
		// Unescape JSON string value
		quoted := append([]byte{'"'}, append(m[1], '"')...)
		var content string
		if err := json.Unmarshal(quoted, &content); err != nil {
			content = string(m[1])
		}
		return map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{"content": content},
				},
			},
			"usage": map[string]interface{}{},
		}, nil
	}

	return nil, fmt.Errorf("failed to parse response after 3 passes: %s", string(raw[:min(200, len(raw))]))
}

// estimateOpenRouterCost applies approximate per-token pricing by model family.
// Rates are indicative — check openrouter.ai/models for current pricing.
func estimateOpenRouterCost(model string, inputTokens, outputTokens int) float64 {
	m := strings.ToLower(model)

	var inputPer1M, outputPer1M float64
	switch {
	case strings.Contains(m, "claude-opus"):
		inputPer1M, outputPer1M = 15.0, 75.0
	case strings.Contains(m, "claude-sonnet"):
		inputPer1M, outputPer1M = 3.0, 15.0
	case strings.Contains(m, "claude-haiku"):
		inputPer1M, outputPer1M = 0.25, 1.25
	case strings.Contains(m, "gpt-4o-mini"):
		inputPer1M, outputPer1M = 0.15, 0.60
	case strings.Contains(m, "gpt-4o"):
		inputPer1M, outputPer1M = 2.50, 10.0
	case strings.Contains(m, "gemini-2.5-pro"):
		inputPer1M, outputPer1M = 1.25, 10.0
	case strings.Contains(m, "gemini"):
		inputPer1M, outputPer1M = 0.10, 0.40
	case strings.Contains(m, "llama"):
		inputPer1M, outputPer1M = 0.10, 0.10
	case strings.Contains(m, "deepseek"):
		inputPer1M, outputPer1M = 0.14, 0.28
	case strings.Contains(m, "mistral"):
		inputPer1M, outputPer1M = 2.0, 6.0
	default:
		inputPer1M, outputPer1M = 1.0, 3.0
	}

	return (float64(inputTokens)*inputPer1M + float64(outputTokens)*outputPer1M) / 1_000_000
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
