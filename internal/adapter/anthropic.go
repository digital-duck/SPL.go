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

// AnthropicAdapter sends requests to the Anthropic Claude API.
type AnthropicAdapter struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	Timeout      time.Duration
	client       *http.Client
}

// NewAnthropicAdapter creates a new AnthropicAdapter from optional config.
func NewAnthropicAdapter(cfg map[string]string) *AnthropicAdapter {
	baseURL := "https://api.anthropic.com"
	if cfg != nil {
		if v, ok := cfg["base_url"]; ok && v != "" {
			baseURL = v
		}
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if cfg != nil {
		if v, ok := cfg["api_key"]; ok && v != "" {
			apiKey = v
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

	timeoutSecs := 180
	timeout := time.Duration(timeoutSecs) * time.Second

	a := &AnthropicAdapter{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		Timeout:      timeout,
	}
	a.client = &http.Client{Timeout: timeout}
	return a
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *AnthropicAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if model == "" {
		model = a.DefaultModel
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}
	if system != "" {
		reqBody.System = system
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := float64(time.Since(start).Milliseconds())

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var anthropicResp anthropicResponse
	if err = json.Unmarshal(respBytes, &anthropicResp); err != nil {
		return nil, fmt.Errorf("anthropic: parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("anthropic: API error %s: %s", anthropicResp.Error.Type, anthropicResp.Error.Message)
	}

	content := ""
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	inputTokens := anthropicResp.Usage.InputTokens
	outputTokens := anthropicResp.Usage.OutputTokens
	if inputTokens == 0 {
		inputTokens = len(prompt) / 4
	}
	if outputTokens == 0 {
		outputTokens = len(content) / 4
	}

	returnedModel := anthropicResp.Model
	if returnedModel == "" {
		returnedModel = model
	}

	costUSD := estimateAnthropicCost(returnedModel, inputTokens, outputTokens)

	return &GenerationResult{
		Content:      content,
		Model:        returnedModel,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		LatencyMs:    latencyMs,
		CostUSD:      costUSD,
	}, nil
}

// estimateAnthropicCost returns estimated cost in USD based on model and token counts.
func estimateAnthropicCost(model string, inputTokens, outputTokens int) float64 {
	// Rates per million tokens
	var inputRate, outputRate float64

	switch {
	case strings.Contains(model, "opus"):
		inputRate = 15.0
		outputRate = 75.0
	case strings.Contains(model, "sonnet"):
		inputRate = 3.0
		outputRate = 15.0
	case strings.Contains(model, "haiku"):
		inputRate = 0.25
		outputRate = 1.25
	default:
		inputRate = 3.0
		outputRate = 15.0
	}

	return (float64(inputTokens)*inputRate + float64(outputTokens)*outputRate) / 1_000_000.0
}

func (a *AnthropicAdapter) CountTokens(text, model string) int {
	return len(text) / 4
}

func (a *AnthropicAdapter) ListModels() []string {
	return []string{"claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5-20251001"}
}

func (a *AnthropicAdapter) Name() string {
	return "anthropic"
}
