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
	"time"
)

// GoogleAdapter sends requests to the Google Gemini API.
type GoogleAdapter struct {
	APIKey       string
	DefaultModel string
	client       *http.Client
}

// NewGoogleAdapter creates a new GoogleAdapter from optional config.
func NewGoogleAdapter(cfg map[string]string) *GoogleAdapter {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if cfg != nil {
		if v, ok := cfg["api_key"]; ok && v != "" {
			apiKey = v
		}
	}

	defaultModel := "gemini-1.5-flash"
	if cfg != nil {
		if v, ok := cfg["model"]; ok && v != "" {
			defaultModel = v
		}
	}

	return &GoogleAdapter{
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		client:       &http.Client{Timeout: 60 * time.Second},
	}
}

// geminiChatRequest is a simplified request body for Gemini API.
type geminiChatRequest struct {
	Contents []geminiContent `json:"contents"`
	System   *geminiContent  `json:"system_instruction,omitempty"`
	Config   geminiConfig    `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string           `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64
}

type geminiConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

func (a *GoogleAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	blocks := []ContentBlock{{Type: "text", Text: prompt}}
	return a.GenerateMultimodal(ctx, blocks, model, maxTokens, temperature, system)
}

func (a *GoogleAdapter) GenerateMultimodal(ctx context.Context, blocks []ContentBlock, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("google: API key not set (set GOOGLE_API_KEY)")
	}
	if model == "" {
		model = a.DefaultModel
	}

	req := geminiChatRequest{
		Config: geminiConfig{
			MaxOutputTokens: maxTokens,
			Temperature:     temperature,
		},
	}

	if system != "" {
		req.System = &geminiContent{Parts: []geminiPart{{Text: system}}}
	}

	var parts []geminiPart
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, geminiPart{Text: b.Text})
		case "image", "audio", "video":
			parts = append(parts, geminiPart{
				InlineData: &geminiInlineData{
					MimeType: b.MediaType,
					Data:     base64.StdEncoding.EncodeToString(b.Data),
				},
			})
		}
	}
	req.Contents = []geminiContent{{Role: "user", Parts: parts}}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, a.APIKey)
	
	bodyBytes, _ := json.Marshal(req)
	start := time.Now()
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	latencyMs := float64(time.Since(start).Milliseconds())
	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	// Very simplified response parsing for brevity
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			PromptTokens     int `json:"promptTokenCount"`
			CandidatesTokens int `json:"candidatesTokenCount"`
			TotalTokens      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, err
	}

	content := ""
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		content = geminiResp.Candidates[0].Content.Parts[0].Text
	}

	return &GenerationResult{
		Content:      content,
		Model:        model,
		InputTokens:  geminiResp.Usage.PromptTokens,
		OutputTokens: geminiResp.Usage.CandidatesTokens,
		TotalTokens:  geminiResp.Usage.TotalTokens,
		LatencyMs:    latencyMs,
	}, nil
}

func (a *GoogleAdapter) CountTokens(text, model string) int {
	return len(text) / 4 // placeholder
}

func (a *GoogleAdapter) ListModels() []string {
	return []string{"gemini-1.5-flash", "gemini-1.5-pro", "gemini-2.0-flash-exp"}
}

func (a *GoogleAdapter) Name() string {
	return "google"
}
