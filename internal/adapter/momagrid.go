package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// MomagridAdapter dispatches tasks to the Momagrid distributed LLM hub.
type MomagridAdapter struct {
	HubURL       string
	DefaultModel string
	Timeout      time.Duration
	PollInterval time.Duration
	MinTier      string
	APIKey       string
	client       *http.Client
}

// NewMomagridAdapter creates a new MomagridAdapter from optional config.
func NewMomagridAdapter(cfg map[string]string) *MomagridAdapter {
	hubURL := "http://localhost:9000"
	if v := os.Getenv("MOMAGRID_HUB_URL"); v != "" {
		hubURL = v
	}
	if cfg != nil {
		if v, ok := cfg["hub_url"]; ok && v != "" {
			hubURL = v
		}
	}

	apiKey := os.Getenv("MOMAGRID_API_KEY")
	if cfg != nil {
		if v, ok := cfg["api_key"]; ok && v != "" {
			apiKey = v
		}
	}

	defaultModel := "llama3.2"
	if cfg != nil {
		if v, ok := cfg["model"]; ok && v != "" {
			defaultModel = v
		}
	}

	minTier := "BRONZE"
	if cfg != nil {
		if v, ok := cfg["min_tier"]; ok && v != "" {
			minTier = v
		}
	}

	timeout := 300 * time.Second
	pollInterval := 2 * time.Second

	a := &MomagridAdapter{
		HubURL:       hubURL,
		DefaultModel: defaultModel,
		Timeout:      timeout,
		PollInterval: pollInterval,
		MinTier:      minTier,
		APIKey:       apiKey,
	}
	a.client = &http.Client{Timeout: timeout}
	return a
}

type momagridTaskRequest struct {
	TaskID      string  `json:"task_id"`
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	System      string  `json:"system,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MinTier     string  `json:"min_tier,omitempty"`
	TimeoutS    int     `json:"timeout_s,omitempty"`
	Priority    int     `json:"priority"`
}

type momagridTaskResult struct {
	Content    string  `json:"content"`
	Error      string  `json:"error"`
	InputToks  int     `json:"input_tokens"`
	OutputToks int     `json:"output_tokens"`
	LatencyMs  float64 `json:"latency_ms"`
	Model      string  `json:"model"`
	AgentName  string  `json:"agent_name"`
}

type momagridTaskResponse struct {
	TaskID string             `json:"task_id"`
	State  string             `json:"state"`
	Result momagridTaskResult `json:"result"`
	Error  string             `json:"error"`
}

func (a *MomagridAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, system string) (*GenerationResult, error) {
	if model == "" {
		model = a.DefaultModel
	}
	if maxTokens == 0 {
		maxTokens = 1000
	}
	if temperature == 0 {
		temperature = 0.7
	}

	taskID := uuid.New().String()
	reqBody := momagridTaskRequest{
		TaskID:      taskID,
		Model:       model,
		Prompt:      prompt,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		MinTier:     a.MinTier,
		TimeoutS:    int(a.Timeout.Seconds()),
		Priority:    1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("momagrid: marshal task request: %w", err)
	}

	start := time.Now()

	// Submit the task
	req, err := http.NewRequestWithContext(ctx, "POST", a.HubURL+"/tasks", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("momagrid: create task request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.APIKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("momagrid: submit task: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("momagrid: submit task HTTP %d", resp.StatusCode)
	}

	// Poll for completion
	deadline := time.Now().Add(a.Timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(a.PollInterval):
		}

		pollReq, err := http.NewRequestWithContext(ctx, "GET", a.HubURL+"/tasks/"+taskID, nil)
		if err != nil {
			return nil, fmt.Errorf("momagrid: create poll request: %w", err)
		}
		if a.APIKey != "" {
			pollReq.Header.Set("Authorization", "Bearer "+a.APIKey)
		}

		pollResp, err := a.client.Do(pollReq)
		if err != nil {
			continue // retry on transient error
		}

		pollBodyBytes, err := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()
		if err != nil {
			continue
		}

		var taskResp momagridTaskResponse
		if err = json.Unmarshal(pollBodyBytes, &taskResp); err != nil {
			continue
		}

		switch taskResp.State {
		case "COMPLETE":
			latencyMs := taskResp.Result.LatencyMs
			if latencyMs == 0 {
				latencyMs = float64(time.Since(start).Milliseconds())
			}
			inputToks := taskResp.Result.InputToks
			outputToks := taskResp.Result.OutputToks
			if inputToks == 0 {
				inputToks = len(prompt) / 4
			}
			if outputToks == 0 {
				outputToks = len(taskResp.Result.Content) / 4
			}
			return &GenerationResult{
				Content:      taskResp.Result.Content,
				Model:        model,
				InputTokens:  inputToks,
				OutputTokens: outputToks,
				TotalTokens:  inputToks + outputToks,
				LatencyMs:    latencyMs,
				CostUSD:      0.0,
			}, nil

		case "FAILED":
			errMsg := taskResp.Result.Error
			if errMsg == "" {
				errMsg = taskResp.Error
			}
			return nil, fmt.Errorf("momagrid: task failed: %s", errMsg)

		// Active states: PENDING, DISPATCHED, IN_FLIGHT, FORWARDED — keep polling
		default:
			continue
		}
	}

	return nil, fmt.Errorf("momagrid: task %s timed out after %v", taskID, a.Timeout)
}

func (a *MomagridAdapter) CountTokens(text, model string) int {
	return len(text) / 4
}

type momagridAgentsResponse struct {
	Agents []struct {
		Models []string `json:"models"`
	} `json:"agents"`
}

func (a *MomagridAdapter) ListModels() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", a.HubURL+"/agents", nil)
	if err != nil {
		return a.fallbackModels()
	}
	if a.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.APIKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return a.fallbackModels()
	}
	defer resp.Body.Close()

	var agentsResp momagridAgentsResponse
	if err = json.NewDecoder(resp.Body).Decode(&agentsResp); err != nil {
		return a.fallbackModels()
	}

	seen := make(map[string]bool)
	var models []string
	for _, agent := range agentsResp.Agents {
		for _, m := range agent.Models {
			if !seen[m] {
				seen[m] = true
				models = append(models, m)
			}
		}
	}
	if len(models) == 0 {
		return a.fallbackModels()
	}
	return models
}

func (a *MomagridAdapter) fallbackModels() []string {
	return []string{"llama3.2", "mistral", "gemma3"}
}

func (a *MomagridAdapter) Name() string {
	return "momagrid"
}
