package adapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OllamaAdapter sends requests to a local Ollama server.
type OllamaAdapter struct {
	BaseURL      string
	DefaultModel string
	Timeout      time.Duration
	client       *http.Client
}

// NewOllamaAdapter creates a new OllamaAdapter from optional config.
func NewOllamaAdapter(cfg map[string]string) *OllamaAdapter {
	baseURL := "http://localhost:11434"
	if v := os.Getenv("OLLAMA_BASE_URL"); v != "" {
		baseURL = v
	}
	if cfg != nil {
		if v, ok := cfg["base_url"]; ok && v != "" {
			baseURL = v
		}
	}

	defaultModel := "llama3.2"
	if cfg != nil {
		if v, ok := cfg["model"]; ok && v != "" {
			defaultModel = v
		}
	}

	timeout := 120 * time.Second
	a := &OllamaAdapter{
		BaseURL:      baseURL,
		DefaultModel: defaultModel,
		Timeout:      timeout,
	}
	a.client = &http.Client{Timeout: timeout}
	return a
}

// ollamaChatRequest is the request body for the Ollama chat completions API.
type ollamaChatRequest struct {
	Model       string              `json:"model"`
	Messages    []ollamaChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
}

type ollamaChatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content,omitempty"`
	Images  []string `json:"images,omitempty"` // Base64 encoded images for Ollama native/OpenAI-compat
}

type ollamaChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

func (a *OllamaAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	blocks := []ContentBlock{{Type: "text", Text: prompt}}
	return a.GenerateMultimodal(ctx, blocks, model, maxTokens, temperature, system)
}

func (a *OllamaAdapter) GenerateMultimodal(ctx context.Context, blocks []ContentBlock, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if model == "" {
		model = a.DefaultModel
	}
	if maxTokens == 0 {
		maxTokens = 1000
	}
	if temperature == 0 {
		temperature = 0.7
	}

	var messages []ollamaChatMessage
	if system != "" {
		messages = append(messages, ollamaChatMessage{Role: "system", Content: system})
	}

	userMsg := ollamaChatMessage{Role: "user"}
	var textParts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "image":
			userMsg.Images = append(userMsg.Images, base64.StdEncoding.EncodeToString(b.Data))
		}
	}
	userMsg.Content = strings.Join(textParts, "\n")
	messages = append(messages, userMsg)

	reqBody := ollamaChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	start := time.Now()
	// Use Ollama's OpenAI-compatible endpoint or native /api/chat
	// Ollama 0.1.34+ supports 'images' in the OpenAI-compatible /v1/chat/completions messages
	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := float64(time.Since(start).Milliseconds())

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp ollamaChatResponse
	if err = json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("ollama: parse response: %w", err)
	}

	content := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
	}

	inputTokens := chatResp.Usage.PromptTokens
	outputTokens := chatResp.Usage.CompletionTokens
	if inputTokens == 0 {
		inputTokens = a.CountTokens(userMsg.Content, model)
	}
	if outputTokens == 0 {
		outputTokens = a.CountTokens(content, model)
	}

	returnedModel := chatResp.Model
	if returnedModel == "" {
		returnedModel = model
	}

	return &GenerationResult{
		Content:      content,
		Model:        returnedModel,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		LatencyMs:    latencyMs,
		CostUSD:      0.0,
	}, nil
}

func (a *OllamaAdapter) CountTokens(text, model string) int {
	return len(text) / 4
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func (a *OllamaAdapter) ListModels() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", a.BaseURL+"/api/tags", nil)
	if err != nil {
		return a.fallbackModels()
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return a.fallbackModels()
	}
	defer resp.Body.Close()

	var tagsResp ollamaTagsResponse
	if err = json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return a.fallbackModels()
	}

	var names []string
	for _, m := range tagsResp.Models {
		names = append(names, m.Name)
	}
	if len(names) == 0 {
		return a.fallbackModels()
	}
	return names
}

func (a *OllamaAdapter) fallbackModels() []string {
	return []string{"llama3.2", "mistral", "gemma3", "phi3", "deepseek-r1", "qwen2.5"}
}

func (a *OllamaAdapter) Name() string {
	return "ollama"
}
