// Package tokencount provides model-aware token counting and cost estimation.
// Mirrors spl/token_counter.py from the Python SPL runtime.
package tokencount

import (
	"strings"
	"unicode"
)

// modelCPT maps model family prefix to chars-per-token ratio.
var modelCPT = map[string]float64{
	"claude":   3.5,
	"gpt":      4.0,
	"o1":       4.0,
	"o3":       4.0,
	"gemini":   3.8,
	"llama":    3.5,
	"mistral":  3.5,
	"deepseek": 3.5,
	"qwen":     3.5,
}

// modelPricing maps model name substring to (inputPer1M, outputPer1M) in USD.
var modelPricing = map[string][2]float64{
	"claude-opus":          {15.0, 75.0},
	"claude-sonnet":        {3.0, 15.0},
	"claude-haiku":         {0.25, 1.25},
	"gpt-4o-mini":          {0.15, 0.60},
	"gpt-4o":               {2.5, 10.0},
	"gpt-4-turbo":          {10.0, 30.0},
	"gpt-4":                {30.0, 60.0},
	"gpt-3.5":              {0.5, 1.5},
	"o1-mini":              {3.0, 12.0},
	"o1":                   {15.0, 60.0},
	"o3-mini":              {1.1, 4.4},
	"o3":                   {10.0, 40.0},
	"deepseek-reasoner":    {0.55, 2.19},
	"deepseek-chat":        {0.27, 1.10},
	"deepseek":             {0.27, 1.10},
	"qwen-max":             {1.6, 6.4},
	"qwen-turbo":           {0.06, 0.20},
	"qwen-plus":            {0.4, 1.2},
	"qwen":                 {0.4, 1.2},
	"gemini-2.5-pro":       {1.25, 10.0},
	"gemini-2.5-flash":     {0.15, 0.60},
	"gemini-2.0-flash":     {0.10, 0.40},
	"gemini-1.5-pro":       {1.25, 5.0},
	"gemini-1.5-flash":     {0.075, 0.30},
}

// Counter provides token counting and cost estimation for a given model.
type Counter struct {
	model string
}

// New creates a Counter for the given model name.
func New(model string) *Counter {
	return &Counter{model: strings.ToLower(model)}
}

// charsPerToken returns the chars-per-token ratio for the counter's model.
func (c *Counter) charsPerToken() float64 {
	m := c.model
	for family, cpt := range modelCPT {
		if strings.Contains(m, family) {
			return cpt
		}
	}
	return 4.0 // default fallback
}

// Count returns an approximate token count for the given text.
func (c *Counter) Count(text string) int {
	if text == "" {
		return 0
	}
	cpt := c.charsPerToken()
	count := int(float64(len(text)) / cpt)
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}

// TruncateToTokens truncates text to approximately maxTokens tokens,
// trying to break at sentence/newline/space boundaries.
func (c *Counter) TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	cpt := c.charsPerToken()
	maxChars := int(float64(maxTokens) * cpt)
	if len(text) <= maxChars {
		return text
	}

	// Try to find a good break point near maxChars, searching backwards
	candidate := text[:maxChars]

	// Prefer sentence boundary (. ! ?)
	for i := len(candidate) - 1; i >= len(candidate)/2; i-- {
		ch := rune(candidate[i])
		if ch == '.' || ch == '!' || ch == '?' {
			return candidate[:i+1]
		}
	}

	// Try newline boundary
	if idx := strings.LastIndex(candidate, "\n"); idx > len(candidate)/2 {
		return candidate[:idx]
	}

	// Try space boundary
	for i := len(candidate) - 1; i >= len(candidate)/2; i-- {
		if unicode.IsSpace(rune(candidate[i])) {
			return strings.TrimRight(candidate[:i], " \t")
		}
	}

	// Hard truncate
	return candidate
}

// EstimateCost returns the estimated USD cost for inputTokens + outputTokens.
// Returns 0.0 if the model is not in the pricing table.
func (c *Counter) EstimateCost(inputTokens, outputTokens int) float64 {
	m := c.model

	// Search pricing table — more specific matches first (longer key wins)
	bestLen := 0
	var bestPrices [2]float64
	found := false

	for key, prices := range modelPricing {
		if strings.Contains(m, key) && len(key) > bestLen {
			bestLen = len(key)
			bestPrices = prices
			found = true
		}
	}

	if !found {
		return 0.0
	}

	return (float64(inputTokens)*bestPrices[0] + float64(outputTokens)*bestPrices[1]) / 1_000_000
}
