package llm

import (
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
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
	TotalCalls   int       `yaml:"total_calls"`   // Total number of calls made
	InputTokens  int       `yaml:"input_tokens"`  // Total input tokens used
	OutputTokens int       `yaml:"output_tokens"` // Total output tokens used
	TotalCost    float64   `yaml:"total_cost"`    // Total cost in dollars
	LastUsed     time.Time `yaml:"last_used"`     // Last time the LLM was used
}

var (
	// Track token usage per user
	tokenUsage      = make(map[int]*TokenUsage) // Map userId -> TokenUsage
	tokenUsageMutex sync.RWMutex
)

// LoadTokenUsageFromPlayer loads token usage stats from a player's saved data
func LoadTokenUsageFromPlayer(userId int) {
	tokenUsageMutex.Lock()
	defer tokenUsageMutex.Unlock()

	// Skip if already loaded
	if _, exists := tokenUsage[userId]; exists {
		return
	}

	// Get player
	player := users.GetByUserId(userId)
	if player == nil {
		mudlog.Debug("LLM", "debug", "Cannot load token usage for player: player not found", "userId", userId)
		return
	}

	// Get token usage from player data
	llmData := player.GetTempData("LLMUsage")
	if llmData == nil {
		// Also check ConfigOptions directly
		if player.ConfigOptions != nil {
			if cfgData, ok := player.ConfigOptions["LLMUsage"]; ok {
				llmData = cfgData
				mudlog.Debug("LLM", "debug", "Found token usage in ConfigOptions", "userId", userId)
			}
		}

		// If still no data, initialize with defaults
		if llmData == nil {
			tokenUsage[userId] = &TokenUsage{
				LastUsed: time.Time{}, // Zero value
			}
			return
		}
	}

	// Process map data to TokenUsage
	if usageMap, ok := llmData.(map[string]interface{}); ok {
		processUsageMap(userId, usageMap)
	} else if usageMap, ok := llmData.(map[interface{}]interface{}); ok {
		// Handle YAML unmarshalled format (keys are interface{})
		convertedMap := make(map[string]interface{})
		for k, v := range usageMap {
			if ks, ok := k.(string); ok {
				convertedMap[ks] = v
			}
		}
		processUsageMap(userId, convertedMap)
	}
}

// Helper function to process usage map data
func processUsageMap(userId int, usageMap map[string]interface{}) {
	usage := &TokenUsage{}

	if calls, ok := usageMap["total_calls"].(int); ok {
		usage.TotalCalls = calls
	}
	if input, ok := usageMap["input_tokens"].(int); ok {
		usage.InputTokens = input
	}
	if output, ok := usageMap["output_tokens"].(int); ok {
		usage.OutputTokens = output
	}
	if cost, ok := usageMap["total_cost"].(float64); ok {
		usage.TotalCost = cost
	}
	if lastUsed, ok := usageMap["last_used"].(time.Time); ok {
		usage.LastUsed = lastUsed
	}

	tokenUsage[userId] = usage
	mudlog.Debug("LLM", "debug", "Loaded token usage for player", "userId", userId, "calls", usage.TotalCalls, "inputTokens", usage.InputTokens)
}

// SaveTokenUsageToPlayer saves the current token usage to the player's data
func SaveTokenUsageToPlayer(userId int) {
	tokenUsageMutex.RLock()
	usage, exists := tokenUsage[userId]
	tokenUsageMutex.RUnlock()

	if !exists || usage == nil {
		return
	}

	// Get player
	player := users.GetByUserId(userId)
	if player == nil {
		mudlog.Debug("LLM", "debug", "Cannot save token usage: player not found", "userId", userId)
		return
	}

	// Store token usage in player data
	player.SetTempData("LLMUsage", map[string]interface{}{
		"total_calls":   usage.TotalCalls,
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
		"total_cost":    usage.TotalCost,
		"last_used":     usage.LastUsed,
	})

	mudlog.Debug("LLM", "debug", "Saved token usage for player", "userId", userId, "calls", usage.TotalCalls, "tokens", usage.InputTokens+usage.OutputTokens)
}

// RecordTokenUsage records token usage for a user
func RecordTokenUsage(userId int, model string, inputTokens, outputTokens int) {
	tokenUsageMutex.Lock()
	defer tokenUsageMutex.Unlock()

	// Load usage from player data if not already loaded
	if _, exists := tokenUsage[userId]; !exists {
		tokenUsageMutex.Unlock()
		LoadTokenUsageFromPlayer(userId)
		tokenUsageMutex.Lock()
	}

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

	// Save updated usage to player data
	tokenUsageMutex.Unlock()
	SaveTokenUsageToPlayer(userId)
	tokenUsageMutex.Lock()
}

// GetTokenUsage gets token usage for a user
func GetTokenUsage(userId int) *TokenUsage {
	tokenUsageMutex.RLock()
	defer tokenUsageMutex.RUnlock()

	// Load usage from player data if not already loaded
	if _, exists := tokenUsage[userId]; !exists {
		tokenUsageMutex.RUnlock()
		LoadTokenUsageFromPlayer(userId)
		tokenUsageMutex.RLock()
	}

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

// We need to also save token usage when a player logs out or the server shuts down
func SaveAllTokenUsage() {
	tokenUsageMutex.RLock()
	defer tokenUsageMutex.RUnlock()

	for userId := range tokenUsage {
		tokenUsageMutex.RUnlock()
		SaveTokenUsageToPlayer(userId)
		tokenUsageMutex.RLock()
	}
}
