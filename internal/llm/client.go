package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// LLMMessage represents a message in the conversation
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

// LLMRequest represents a request to a generic OpenAI-compatible API
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

// callLLM sends a request to the LLM API and returns the response
func callLLM(config HelpLLMConfig, messages []LLMMessage) (string, error) {
	// Determine if we're using OpenAI by checking if the URL contains api.openai.com
	if strings.Contains(config.EndpointURL, "api.openai.com") {
		mudlog.Info("callLLM", "using_openai", "Using OpenAI API", "endpoint", config.EndpointURL)
		// Using OpenAI API directly
		return callOpenAICompatibleAPI(config, messages)
	} else if !strings.Contains(config.EndpointURL, "api/chat/completions") {
		mudlog.Info("callLLM", "using_ollama_or_direct", "Using Ollama/direct API logic", "endpoint", config.EndpointURL)
		// First try the standard Ollama chat API (which callOllamaAPI is designed for)
		response, err := callOllamaAPI(config, messages)
		if err == nil {
			return response, nil
		}

		// If that fails, try the simple Ollama completions API
		mudlog.Warn("help-llm", "ollama_chat_api_failed", "Ollama Chat API call failed, trying Ollama completions API as fallback", "err", err.Error())
		return callOllamaCompletionAPI(config, messages)
	} else {
		mudlog.Info("callLLM", "using_openai_compatible", "Using OpenAI-compatible API logic", "endpoint", config.EndpointURL)
		// Using OpenAI-compatible API
		return callOpenAICompatibleAPI(config, messages)
	}
}

// callOllamaAPI calls the Ollama API directly
func callOllamaAPI(config HelpLLMConfig, messages []LLMMessage) (string, error) {
	mudlog.Error("help-llm", "ENTERED_CALLOLLAMAAPI", "Attempting to call Ollama /api/chat")

	// Create the request body for Ollama
	reqBody := OllamaRequest{
		Model:    config.Model,
		Messages: messages,
	}
	reqBody.Options.Temperature = 0.7 // Default temperature

	// Convert request to JSON
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		mudlog.Error("help-llm", "error_ollama_marshal", "Failed to marshal request JSON for Ollama", "err", err)
		return "", fmt.Errorf("error marshaling request for Ollama: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Ensure the correct endpoint URL for Ollama
	ollamaEndpoint := config.EndpointURL
	// Only append /api/chat for Ollama endpoints, not for OpenAI endpoints
	if !strings.Contains(ollamaEndpoint, "api.openai.com") && !strings.HasSuffix(ollamaEndpoint, "/api/chat") {
		ollamaEndpoint = strings.TrimSuffix(ollamaEndpoint, "/") + "/api/chat"
	}

	// Create request
	req, err := http.NewRequest("POST", ollamaEndpoint, bytes.NewBuffer(reqJSON))
	if err != nil {
		mudlog.Error("help-llm", "error_ollama_createreq", "Failed to create HTTP request for Ollama", "endpoint", ollamaEndpoint, "err", err)
		return "", fmt.Errorf("error creating HTTP request for Ollama: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	mudlog.Info("help-llm", "ollama_request_info", "Sending request to Ollama API", "endpoint", ollamaEndpoint, "request_body_snippet", string(reqJSON[:min(len(reqJSON), 200)]))
	resp, err := client.Do(req)
	if err != nil {
		mudlog.Error("help-llm", "error_ollama_sendreq", "Failed to send request to Ollama", "endpoint", ollamaEndpoint, "err", err)
		return "", fmt.Errorf("error sending request to Ollama: %v", err)
	}
	defer resp.Body.Close()

	// Log HTTP status and headers
	mudlog.Info("help-llm", "ollama_response_status", resp.StatusCode, "ollama_headers", resp.Header)

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		mudlog.Error("help-llm", "error_ollama_readbody", "Failed to read Ollama response body", "err", err)
		return "", fmt.Errorf("error reading Ollama response: %v", err)
	}

	// Log the raw response body for debugging (at a lower level)
	mudlog.Info("help-llm", "OLLAMA_RAW_RESPONSE_BODY", string(respBody[:min(len(respBody), 500)]))

	// For Ollama, we know from experience that it uses streaming JSON format,
	// so let's directly parse it that way instead of trying standard format first
	mudlog.Info("help-llm", "ollama_parsing_stream", "Parsing Ollama response as JSON stream")

	// Split the response into lines and process each line as a JSON object
	lines := strings.Split(string(respBody), "\n")
	var fullResponse strings.Builder

	// Add debug logging for the parsing process
	mudlog.Info("help-llm", "stream_parse_start", fmt.Sprintf("Parsing %d lines", len(lines)))

	for i, line := range lines {
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
			mudlog.Warn("help-llm", "ollama_stream_parse_error", "Error decoding Ollama JSON stream object", "err", err, "line", line)
			continue
		}

		if streamResp.Message.Content != "" {
			fullResponse.WriteString(streamResp.Message.Content)
			if i < 5 || i > len(lines)-5 {
				// Log the first and last few tokens for debugging
				mudlog.Info("help-llm", "token", fmt.Sprintf("Line %d: '%s'", i, streamResp.Message.Content))
			}
		}

		if streamResp.Done {
			mudlog.Info("help-llm", "stream_done", "Found 'done' flag in stream")
			break
		}
	}

	if fullResponse.Len() > 0 {
		finalResponse := fullResponse.String()
		mudlog.Info("help-llm", "ollama_success_stream_parse", "Successfully parsed and concatenated Ollama stream",
			"length", fullResponse.Len(),
			"start", finalResponse[:min(30, len(finalResponse))],
			"end", finalResponse[max(0, len(finalResponse)-30):])
		return finalResponse, nil
	}

	// If stream parsing failed, try to parse as a standard Ollama response
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err == nil {
		if ollamaResp.Error != "" {
			mudlog.Warn("help-llm", "ollama_api_error_standard", ollamaResp.Error)
			return "", fmt.Errorf("Ollama API error: %s", ollamaResp.Error)
		}
		if ollamaResp.Message.Content != "" {
			mudlog.Info("help-llm", "ollama_success_standard", "Parsed standard Ollama response")
			return ollamaResp.Message.Content, nil
		}
	} else {
		mudlog.Warn("help-llm", "ollama_parse_error_standard", "Failed to parse as standard OllamaResponse", "err", err)
	}

	// Final fallback: Try to parse as a simple string response from /api/generate
	mudlog.Info("help-llm", "ollama_attempt_generate_parse", "Attempting to parse Ollama as simple /api/generate response")
	var generateResp struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &generateResp); err == nil {
		if generateResp.Error != "" {
			mudlog.Warn("help-llm", "ollama_api_error_generate", generateResp.Error)
			return "", fmt.Errorf("Ollama API error (generate): %s", generateResp.Error)
		}
		if generateResp.Response != "" {
			mudlog.Info("help-llm", "ollama_success_generate", "Parsed Ollama as simple /api/generate response")
			return generateResp.Response, nil
		}
	} else {
		mudlog.Warn("help-llm", "ollama_parse_error_generate", "Failed to parse Ollama as simple /api/generate response", "err", err)
	}

	mudlog.Error("help-llm", "ollama_final_error", "Could not extract valid response from Ollama API after all attempts")
	return "", fmt.Errorf("could not extract valid response from Ollama API, response body: %s", string(respBody[:min(len(respBody), 200)]))
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
func callOpenAICompatibleAPI(config HelpLLMConfig, messages []LLMMessage) (string, error) {
	// Create the request
	reqBody := LLMRequest{
		Model:       config.Model,
		Messages:    messages,
		Temperature: 0.7, // Default temperature
		MaxTokens:   500, // Limit token length for help responses
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

	// Create request
	req, err := http.NewRequest("POST", config.EndpointURL, bytes.NewBuffer(reqJSON))
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

// callOllamaCompletionAPI calls the simpler Ollama completions API as a fallback
func callOllamaCompletionAPI(config HelpLLMConfig, messages []LLMMessage) (string, error) {
	// Combine all the messages into a single prompt
	var prompt strings.Builder
	for _, msg := range messages {
		prompt.WriteString(msg.Role)
		prompt.WriteString(": ")
		prompt.WriteString(msg.Content)
		prompt.WriteString("\n\n")
	}
	prompt.WriteString("assistant: ")

	// Create the request body for Ollama completions
	reqBody := struct {
		Model       string  `json:"model"`
		Prompt      string  `json:"prompt"`
		Stream      bool    `json:"stream"`
		Temperature float64 `json:"temperature"`
	}{
		Model:       config.Model,
		Prompt:      prompt.String(),
		Stream:      false,
		Temperature: 0.7,
	}

	// Convert request to JSON
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 45 * time.Second, // Longer timeout for completions
	}

	// Ensure the correct endpoint URL for Ollama completions
	ollamaEndpoint := config.EndpointURL
	// Only append /api/generate for Ollama endpoints, not for OpenAI endpoints
	if !strings.Contains(ollamaEndpoint, "api.openai.com") {
		ollamaEndpoint = strings.TrimSuffix(ollamaEndpoint, "/") + "/api/generate"
	}

	// Create request
	req, err := http.NewRequest("POST", ollamaEndpoint, bytes.NewBuffer(reqJSON))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	mudlog.Info("llm-ollama", "request", "Sending request to Ollama completions API", "endpoint", ollamaEndpoint)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to Ollama: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Parse the response
	var generateResp struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}

	if err := json.Unmarshal(respBody, &generateResp); err != nil {
		return "", fmt.Errorf("error parsing Ollama completions response: %v", err)
	}

	if generateResp.Error != "" {
		return "", fmt.Errorf("Ollama API error: %s", generateResp.Error)
	}

	return generateResp.Response, nil
}

// GetHelpResponse generates a response to a help query using the LLM
func GetHelpResponse(query string, availableCommands []string, userId ...int) (string, error) {
	if !helpLLMConfig.Enabled {
		return "", fmt.Errorf("LLM help system is not enabled")
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
			Content: helpLLMConfig.SystemPrompt,
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
	response, err := callLLM(helpLLMConfig, messages)
	if err != nil {
		return "", err
	}

	// Track token usage if userId is provided
	if len(userId) > 0 && userId[0] > 0 {
		// Estimate token usage (since we don't have access to the actual token counts from callLLM)
		promptTokens := EstimateTokenCount(messages[0].Content) + EstimateTokenCount(messages[1].Content)
		responseTokens := EstimateTokenCount(response)
		RecordTokenUsage(userId[0], helpLLMConfig.Model, promptTokens, responseTokens)
		mudlog.Info("help-llm", "tokens_recorded",
			fmt.Sprintf("Recorded token usage for user %d: ~%d input, ~%d output",
				userId[0], promptTokens, responseTokens))
	}

	// Save the response as a template file if configured to do so
	if helpLLMConfig.SaveResponses {
		err = saveHelpResponse(query, response)
		if err != nil {
			mudlog.Warn("llm-help", "error", "Failed to save response", "query", query, "err", err)
		}
	}

	return response, nil
}
