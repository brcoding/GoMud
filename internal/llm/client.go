package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// LLMMessage represents a message in the conversation
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMRequest represents a generic request to an LLM API
type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

// LLMResponse represents a response from an OpenAI-compatible API
type LLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// OllamaRequest represents a request to the Ollama API
type OllamaRequest struct {
	Model    string       `json:"model"`
	Messages []LLMMessage `json:"messages"`
	Options  struct {
		Temperature float64 `json:"temperature"`
	} `json:"options"`
}

// OllamaResponse represents a response from the Ollama API
type OllamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error,omitempty"`
}

// LLMConfig defines the configuration for interacting with an LLM
type LLMConfig struct {
	Enabled       bool    `yaml:"enabled"`
	Provider      string  `yaml:"provider"`
	EndpointURL   string  `yaml:"endpoint_url"`
	APIKey        string  `yaml:"api_key"`
	Model         string  `yaml:"model"`
	Temperature   float64 `yaml:"temperature"`
	MaxTokens     int     `yaml:"max_tokens"`
	SystemPrompt  string  `yaml:"system_prompt"`
	SaveResponses bool    `yaml:"save_responses"`
}

// Global LLM configuration
var globalConfig LLMConfig

// Initialize the LLM service with global configuration
func Initialize() {
	// Load config from the main config or use defaults
	intConfig := configs.GetIntegrationsConfig()

	globalConfig = LLMConfig{
		Enabled:       bool(intConfig.LLM.Enabled),
		Provider:      string(intConfig.LLM.Provider),
		EndpointURL:   string(intConfig.LLM.BaseURL),
		APIKey:        string(intConfig.LLMHelp.APIKey), // Use the same API key as help system for now
		Model:         string(intConfig.LLM.Model),
		Temperature:   float64(intConfig.LLM.Temperature),
		MaxTokens:     500,                                   // Default value
		SystemPrompt:  "",                                    // Will be set by each specific use case
		SaveResponses: bool(intConfig.LLMHelp.SaveResponses), // Use the help system's setting
	}

	mudlog.Info("LLM", "info", "LLM service initialized", "enabled", globalConfig.Enabled)
}

// GetStatus returns whether the LLM service is enabled
func GetStatus() bool {
	return globalConfig.Enabled
}

// SendRequest sends a message to the LLM and returns the response
// This is the main entry point for all LLM interactions
func SendRequest(messages []LLMMessage, config ...LLMConfig) (string, error) {
	// Use the provided config or fall back to the global config
	useConfig := globalConfig
	if len(config) > 0 {
		useConfig = config[0]
	}

	// Check if LLM is enabled
	if !useConfig.Enabled {
		return "", fmt.Errorf("LLM service is not enabled")
	}

	// Check which provider to use and route accordingly
	provider := strings.ToLower(useConfig.Provider)
	if provider == "openai" || strings.Contains(useConfig.EndpointURL, "openai.com") {
		return callOpenAICompatibleAPI(useConfig, messages)
	} else if provider == "ollama" || strings.Contains(useConfig.EndpointURL, "ollama") {
		return callOllamaAPI(useConfig, messages)
	} else {
		// Default to OpenAI compatible API
		return callOpenAICompatibleAPI(useConfig, messages)
	}
}

// Helper function to prevent panic with string slicing
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// callOpenAICompatibleAPI calls an OpenAI-compatible API
func callOpenAICompatibleAPI(config LLMConfig, messages []LLMMessage) (string, error) {
	// Create the request
	reqBody := LLMRequest{
		Model:       config.Model,
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
	}

	// Convert request to JSON
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Ensure the endpoint includes the completions path
	endpoint := config.EndpointURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = strings.TrimSuffix(endpoint, "/") + "/chat/completions"
	}

	// Create request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqJSON))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Parse response
	var llmResp LLMResponse
	err = json.Unmarshal(respBody, &llmResp)
	if err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	// Check for error
	if llmResp.Error != nil {
		return "", fmt.Errorf("LLM API error: %s", llmResp.Error.Message)
	}

	// Check if we have a valid response
	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	// Return the response
	return llmResp.Choices[0].Message.Content, nil
}

// callOllamaAPI calls the Ollama API
func callOllamaAPI(config LLMConfig, messages []LLMMessage) (string, error) {
	mudlog.Info("LLM", "debug", "Attempting to call Ollama API")

	// Create the request body for Ollama
	reqBody := OllamaRequest{
		Model:    config.Model,
		Messages: messages,
	}
	reqBody.Options.Temperature = config.Temperature

	// Convert request to JSON
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		mudlog.Error("LLM", "error", "Failed to marshal request JSON for Ollama", "err", err)
		return "", fmt.Errorf("error marshaling request for Ollama: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Ensure the correct endpoint URL for Ollama
	ollamaEndpoint := config.EndpointURL
	if !strings.HasSuffix(ollamaEndpoint, "/api/chat") {
		ollamaEndpoint = strings.TrimSuffix(ollamaEndpoint, "/") + "/api/chat"
	}

	// Create request
	req, err := http.NewRequest("POST", ollamaEndpoint, bytes.NewBuffer(reqJSON))
	if err != nil {
		mudlog.Error("LLM", "error", "Failed to create HTTP request for Ollama", "endpoint", ollamaEndpoint, "err", err)
		return "", fmt.Errorf("error creating HTTP request for Ollama: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	mudlog.Info("LLM", "debug", "Sending request to Ollama API", "endpoint", ollamaEndpoint)
	resp, err := client.Do(req)
	if err != nil {
		mudlog.Error("LLM", "error", "Failed to send request to Ollama", "endpoint", ollamaEndpoint, "err", err)
		return "", fmt.Errorf("error sending request to Ollama: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		mudlog.Error("LLM", "error", "Failed to read Ollama response body", "err", err)
		return "", fmt.Errorf("error reading Ollama response: %v", err)
	}

	// Try to parse as a standard Ollama response first
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err == nil {
		if ollamaResp.Error != "" {
			mudlog.Warn("LLM", "warning", "Ollama API error", "error", ollamaResp.Error)
			return "", fmt.Errorf("Ollama API error: %s", ollamaResp.Error)
		}
		if ollamaResp.Message.Content != "" {
			mudlog.Info("LLM", "debug", "Parsed standard Ollama response")
			return ollamaResp.Message.Content, nil
		}
	}

	// If that fails, try to parse as a stream
	mudlog.Info("LLM", "debug", "Parsing Ollama response as JSON stream")
	lines := strings.Split(string(respBody), "\n")
	var fullResponse strings.Builder

	for _, line := range lines {
		if line == "" {
			continue
		}

		var streamResp struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}

		if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
			continue
		}

		if streamResp.Message.Content != "" {
			fullResponse.WriteString(streamResp.Message.Content)
		}

		if streamResp.Done {
			break
		}
	}

	if fullResponse.Len() > 0 {
		return fullResponse.String(), nil
	}

	// Final fallback for simpler API
	var generateResp struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &generateResp); err == nil {
		if generateResp.Error != "" {
			return "", fmt.Errorf("Ollama API error: %s", generateResp.Error)
		}
		if generateResp.Response != "" {
			return generateResp.Response, nil
		}
	}

	return "", fmt.Errorf("could not extract valid response from Ollama API")
}

// Helper function to create a properly formatted LLM config from various sources
func CreateConfig(baseConfig LLMConfig, overrides ...map[string]interface{}) LLMConfig {
	config := baseConfig

	// Apply any overrides
	if len(overrides) > 0 {
		for _, override := range overrides {
			for key, value := range override {
				switch key {
				case "enabled":
					if enabled, ok := value.(bool); ok {
						config.Enabled = enabled
					}
				case "model":
					if model, ok := value.(string); ok {
						config.Model = model
					}
				case "temperature":
					if temp, ok := value.(float64); ok {
						config.Temperature = temp
					}
				case "max_tokens":
					if tokens, ok := value.(int); ok {
						config.MaxTokens = tokens
					}
				case "system_prompt":
					if prompt, ok := value.(string); ok {
						config.SystemPrompt = prompt
					}
				}
			}
		}
	}

	return config
}

// GetHelpResponse generates a response to a help query using the LLM
func GetHelpResponse(query string, availableCommands []string, userId ...int) (string, error) {
	if !globalConfig.Enabled {
		return "", fmt.Errorf("LLM help system is not enabled")
	}

	// Check if the player has disabled LLM features
	if len(userId) > 0 && userId[0] > 0 && IsLLMDisabledForPlayer(userId[0]) {
		return "", fmt.Errorf("LLM features are disabled for this player")
	}

	// Create a formatted list of commands grouped by type
	var commandsByType = make(map[string][]string)

	for _, cmd := range keywords.GetAllHelpTopicInfo() {
		cmdType := "command"
		if cmd.Type == "skill" {
			cmdType = "skill"
		} else if cmd.AdminOnly {
			cmdType = "admin"
		}

		commandsByType[cmdType] = append(commandsByType[cmdType], cmd.Command)
	}

	// Build a help context with information about available commands
	var contextBuilder strings.Builder
	contextBuilder.WriteString("VERIFIED AVAILABLE COMMANDS LIST:\n")
	contextBuilder.WriteString("===================================\n")
	contextBuilder.WriteString("ONLY suggest commands from this list. DO NOT suggest any commands that aren't listed here.\n\n")

	for cmdType, cmds := range commandsByType {
		contextBuilder.WriteString(fmt.Sprintf("✓ %s commands: %s\n\n",
			strings.Title(cmdType),
			strings.Join(cmds, ", ")))
	}

	contextBuilder.WriteString("===================================\n")
	contextBuilder.WriteString("Remember: Players can ONLY use the commands listed above. Never suggest a command that isn't in this list.\n")

	// Create messages for the LLM API
	messages := []LLMMessage{
		{
			Role:    "system",
			Content: globalConfig.SystemPrompt,
		},
		{
			Role: "user",
			Content: fmt.Sprintf("I need help with: %s\n\n%s",
				query,
				contextBuilder.String()),
		},
	}

	mudlog.Info("GetHelpResponse", "messages", messages)

	// Get response from LLM API
	response, err := SendRequest(messages, globalConfig)
	if err != nil {
		return "", err
	}

	// Track token usage if userId is provided
	if len(userId) > 0 && userId[0] > 0 {
		// Estimate token usage (since we don't have access to the actual token counts from SendRequest)
		promptTokens := EstimateTokenCount(messages[0].Content) + EstimateTokenCount(messages[1].Content)
		responseTokens := EstimateTokenCount(response)
		RecordTokenUsage(userId[0], globalConfig.Model, promptTokens, responseTokens)
		mudlog.Info("help-llm", "tokens_recorded",
			fmt.Sprintf("Recorded token usage for user %d: ~%d input, ~%d output",
				userId[0], promptTokens, responseTokens))
	}

	// Save the response as a template file if configured to do so
	if globalConfig.SaveResponses {
		err = saveHelpResponse(query, response)
		if err != nil {
			mudlog.Warn("llm-help", "error", "Failed to save response", "query", query, "err", err)
		}
	}

	return response, nil
}
