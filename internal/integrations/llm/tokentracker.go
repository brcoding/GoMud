package llm

import (
	"sync"
	"time"
)

// Cost per 1K tokens for different models
const (
	// OpenAI models
	CostPerGPT41Nano_Input  = 0.0003 // $0.0003 per 1K input tokens
	CostPerGPT41Nano_Output = 0.0015 // $0.0015 per 1K output tokens

	// Ollama costs are assumed to be zero when run locally
	CostPerOllamaToken = 0.0 // $0.00 per 1K tokens (local)
)

// TokenUsage holds token usage stats for a user
type TokenUsage struct {
	TotalCalls   int       // Total number of calls made
	InputTokens  int       // Total input tokens used
	OutputTokens int       // Total output tokens used
	TotalCost    float64   // Total cost in dollars
	LastUsed     time.Time // Last time the LLM was used
}

var (
	// Track token usage per user
	tokenUsage      = make(map[int]*TokenUsage) // Map userId -> TokenUsage
	tokenUsageMutex sync.RWMutex
)

// RecordTokenUsage records token usage for a user
func RecordTokenUsage(userId int, model string, inputTokens, outputTokens int) {
	tokenUsageMutex.Lock()
	defer tokenUsageMutex.Unlock()

	// Create usage record if not exists
	if _, exists := tokenUsage[userId]; !exists {
		tokenUsage[userId] = &TokenUsage{
			LastUsed: time.Now(),
		}
	}

	// Update stats
	usage := tokenUsage[userId]
	usage.TotalCalls++
	usage.InputTokens += inputTokens
	usage.OutputTokens += outputTokens
	usage.LastUsed = time.Now()

	// Calculate cost based on model
	inputCost := 0.0
	outputCost := 0.0

	if model == "gpt-4.1-nano" {
		inputCost = float64(inputTokens) * CostPerGPT41Nano_Input / 1000.0
		outputCost = float64(outputTokens) * CostPerGPT41Nano_Output / 1000.0
	}
	// Add more models as needed

	usage.TotalCost += inputCost + outputCost
}

// GetTokenUsage gets token usage for a user
func GetTokenUsage(userId int) *TokenUsage {
	tokenUsageMutex.RLock()
	defer tokenUsageMutex.RUnlock()

	if usage, exists := tokenUsage[userId]; exists {
		return usage
	}

	// Return empty usage if not found
	return &TokenUsage{
		LastUsed: time.Time{}, // Zero value
	}
}

// EstimateTokenCount gives a rough estimate of token count based on text length
// This is a very rough estimate - for accurate counts you need a proper tokenizer
func EstimateTokenCount(text string) int {
	// Rough estimate: 1 token ≈ 4 characters in English
	return len(text) / 4
}
