// Package config handles SPL runtime configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// AdapterConfig holds per-adapter configuration.
type AdapterConfig struct {
	BaseURL      string `yaml:"base_url"`
	APIKey       string `yaml:"api_key"`
	DefaultModel string `yaml:"default_model"`
	TimeoutSecs  int    `yaml:"timeout_secs"`
	CLIPath      string `yaml:"cli_path"` // for claude_cli
}

// Text2SPLConfig holds configuration for the text2spl compiler.
type Text2SPLConfig struct {
	Adapter    string `yaml:"adapter"`
	Model      string `yaml:"model"`
	Mode       string `yaml:"mode"`        // auto, prompt, workflow
	Validate   bool   `yaml:"validate"`
	MaxRetries int    `yaml:"max_retries"`
}

// CodeRAGConfig holds configuration for the Code-RAG store.
type CodeRAGConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ChromaURL   string `yaml:"chroma_url"`
	OllamaURL   string `yaml:"ollama_url"`
	EmbedModel  string `yaml:"embed_model"`
	Collection  string `yaml:"collection"`
	TopK        int    `yaml:"top_k"`
	AutoCapture bool   `yaml:"auto_capture"`
}

// DocRAGConfig holds configuration for the Doc-RAG store.
type DocRAGConfig struct {
	ChromaURL  string `yaml:"chroma_url"`
	OllamaURL  string `yaml:"ollama_url"`
	EmbedModel string `yaml:"embed_model"`
	Collection string `yaml:"collection"`
}

// Config holds the SPL runtime configuration.
type Config struct {
	Adapter        string                   `yaml:"adapter"`
	Model          string                   `yaml:"model"`
	MaxLLMCalls    int                      `yaml:"max_llm_calls"`
	MaxTotalTokens int                      `yaml:"max_total_tokens"`
	Adapters       map[string]AdapterConfig `yaml:"adapters"`
	Text2SPL       Text2SPLConfig           `yaml:"text2spl"`
	CodeRAG        CodeRAGConfig            `yaml:"code_rag"`
	DocRAG         DocRAGConfig             `yaml:"doc_rag"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Adapter:        "ollama",
		Model:          "",
		MaxLLMCalls:    100,
		MaxTotalTokens: 500000,
		Adapters: map[string]AdapterConfig{
			"ollama": {
				BaseURL:      "http://localhost:11434",
				DefaultModel: "llama3.2",
				TimeoutSecs:  120,
			},
			"anthropic": {
				BaseURL:      "https://api.anthropic.com",
				DefaultModel: "claude-sonnet-4-6",
				TimeoutSecs:  180,
			},
			"claude_cli": {
				DefaultModel: "claude-sonnet-4-6",
				TimeoutSecs:  300,
				CLIPath:      "claude",
			},
			"momagrid": {
				BaseURL:     "http://localhost:9000",
				TimeoutSecs: 600,
			},
			"openrouter": {
				BaseURL:      "https://openrouter.ai/api/v1/chat/completions",
				DefaultModel: "anthropic/claude-sonnet-4-5",
				TimeoutSecs:  180,
			},
			"openai": {
				BaseURL:      "https://api.openai.com/v1/chat/completions",
				DefaultModel: "gpt-4o",
				TimeoutSecs:  180,
			},
			"deepseek": {
				BaseURL:      "https://api.deepseek.com/v1/chat/completions",
				DefaultModel: "deepseek-chat",
				TimeoutSecs:  180,
			},
			"qwen": {
				BaseURL:      "https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions",
				DefaultModel: "qwen-plus",
				TimeoutSecs:  180,
			},
		},
		Text2SPL: Text2SPLConfig{
			Adapter:    "claude_cli",
			Model:      "claude-sonnet-4-6",
			Mode:       "auto",
			Validate:   true,
			MaxRetries: 2,
		},
		CodeRAG: CodeRAGConfig{
			Enabled:     true,
			ChromaURL:   "http://localhost:8000",
			OllamaURL:   "http://localhost:11434",
			EmbedModel:  "nomic-embed-text",
			Collection:  "spl_code_rag",
			TopK:        4,
			AutoCapture: true,
		},
		DocRAG: DocRAGConfig{
			ChromaURL:  "http://localhost:8000",
			OllamaURL:  "http://localhost:11434",
			EmbedModel: "nomic-embed-text",
			Collection: "spl_doc_rag",
		},
	}
}

// Path returns the path to the config file.
func (c *Config) Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".spl", "config.yaml")
}

// Save writes the config to ~/.spl/config.yaml.
func (c *Config) Save() error {
	p := c.Path()
	if p == "" {
		return fmt.Errorf("config: cannot determine home directory")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("config: create directory: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(p, data, 0644); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	return nil
}

// Get returns a top-level config value as string.
func (c *Config) Get(key string) string {
	switch key {
	case "adapter":
		return c.Adapter
	case "model":
		return c.Model
	case "max_llm_calls":
		return strconv.Itoa(c.MaxLLMCalls)
	case "max_total_tokens":
		return strconv.Itoa(c.MaxTotalTokens)
	default:
		return ""
	}
}

// Set sets a top-level config key from a string value.
func (c *Config) Set(key, value string) error {
	switch key {
	case "adapter":
		c.Adapter = value
	case "model":
		c.Model = value
	case "max_llm_calls":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("config: max_llm_calls must be an integer: %w", err)
		}
		c.MaxLLMCalls = n
	case "max_total_tokens":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("config: max_total_tokens must be an integer: %w", err)
		}
		c.MaxTotalTokens = n
	default:
		return fmt.Errorf("config: unknown key %q (valid: adapter, model, max_llm_calls, max_total_tokens)", key)
	}
	return nil
}

// Load loads configuration from ~/.spl/config.yaml.
// Returns default config if the file doesn't exist.
func Load() (*Config, error) {
	cfg := Default()
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	cfgPath := filepath.Join(home, ".spl", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, nil
	}

	if err = yaml.Unmarshal(data, cfg); err != nil {
		return Default(), nil
	}
	return cfg, nil
}

// AdapterConfig returns adapter-specific config as map[string]string.
func (c *Config) AdapterConfig(name string) map[string]string {
	result := make(map[string]string)
	if c.Adapters == nil {
		return result
	}
	ac, ok := c.Adapters[name]
	if !ok {
		return result
	}
	if ac.BaseURL != "" {
		result["base_url"] = ac.BaseURL
	}
	if ac.APIKey != "" {
		result["api_key"] = ac.APIKey
	}
	if ac.DefaultModel != "" {
		result["default_model"] = ac.DefaultModel
	}
	if ac.TimeoutSecs != 0 {
		result["timeout_secs"] = strconv.Itoa(ac.TimeoutSecs)
	}
	if ac.CLIPath != "" {
		result["cli_path"] = ac.CLIPath
	}
	return result
}
