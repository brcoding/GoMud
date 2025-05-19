package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

var (
	initialized bool
	waitMutex   sync.RWMutex
	waitUntil   time.Time
	client      *http.Client
)

const (
	RequestFailureBackoffSeconds = 30
)

// LLMResponse represents a response from the LLM service
type LLMResponse struct {
	Text     string
	Error    error
	Duration time.Duration
}

// Initialize the LLM service
func Init() {
	if initialized {
		return
	}

	client = &http.Client{
		Timeout: 30 * time.Second,
	}

	initialized = true
	mudlog.Info("LLM", "info", "integration initialized")
}

// GenerateResponse sends a prompt to the LLM service and returns the response
func GenerateResponse(prompt string, context []string, userId ...int) LLMResponse {
	if !initialized {
		mudlog.Error("LLM", "error", "LLM service not initialized")
		return LLMResponse{Error: fmt.Errorf("LLM service not initialized")}
	}

	if isRequestBackoff() {
		mudlog.Warn("LLM", "info", "LLM service is in backoff")
		return LLMResponse{Error: fmt.Errorf("LLM service is in backoff")}
	}

	config := configs.GetIntegrationsConfig().LLM
	if !bool(config.Enabled) {
		mudlog.Error("LLM", "error", "LLM integration is disabled")
		return LLMResponse{Error: fmt.Errorf("LLM integration is disabled")}
	}

	mudlog.Info("LLM", "request", fmt.Sprintf("Sending request to LLM with model %s", config.Model))

	start := time.Now()

	// Build the full prompt with context
	fullPrompt := ""
	if len(context) > 0 {
		fullPrompt = strings.Join(context, "\n") + "\n\n"
	}
	fullPrompt += prompt

	// Prepare the request payload based on provider
	var payload map[string]interface{}

	if strings.Contains(string(config.BaseURL), "api.openai.com") {
		// OpenAI API format
		payload = map[string]interface{}{
			"model": string(config.Model),
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are a helpful AI assistant in a fantasy MUD game.",
				},
				{
					"role":    "user",
					"content": fullPrompt,
				},
			},
			"temperature": float64(config.Temperature),
			"max_tokens":  300,
		}
	} else {
		// Ollama API format
		payload = map[string]interface{}{
			"model":       string(config.Model),
			"prompt":      fullPrompt,
			"temperature": float64(config.Temperature),
			"stream":      false,
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		mudlog.Error("LLM", "error", fmt.Sprintf("Failed to marshal request: %v", err))
		return LLMResponse{Error: fmt.Errorf("failed to marshal request: %v", err)}
	}

	// Determine the correct URL based on provider
	var url string
	if strings.Contains(string(config.BaseURL), "api.openai.com") {
		// For OpenAI, use the chat completions endpoint
		url = fmt.Sprintf("%s/chat/completions", string(config.BaseURL))
	} else {
		// For Ollama, append the /api/generate path
		url = fmt.Sprintf("%s/api/generate", string(config.BaseURL))
	}
	mudlog.Debug("LLM", "request", fmt.Sprintf("Sending request to %s", url))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		mudlog.Error("LLM", "error", fmt.Sprintf("Failed to create request: %v", err))
		return LLMResponse{Error: fmt.Errorf("failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")

	// Add Authorization header for OpenAI if needed
	// Note: For OpenAI with NPC system, API key should be set in OPENAI_API_KEY environment variable
	// Or it can be handled by the http client configuration elsewhere
	if strings.Contains(string(config.BaseURL), "api.openai.com") {
		// Get the APIKey from LLMHelp config as a fallback
		helpConfig := configs.GetIntegrationsConfig().LLMHelp
		apiKey := string(helpConfig.APIKey)
		if apiKey != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		mudlog.Error("LLM", "error", fmt.Sprintf("Request failed: %v", err))
		doRequestBackoff()
		return LLMResponse{Error: fmt.Errorf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		mudlog.Error("LLM", "error", fmt.Sprintf("Unexpected status code: %d, body: %s", resp.StatusCode, string(body)))
		doRequestBackoff()
		return LLMResponse{Error: fmt.Errorf("unexpected status code: %d", resp.StatusCode)}
	}

	// Parse the response based on the provider
	var responseText string
	var inputTokens, outputTokens int

	if strings.Contains(string(config.BaseURL), "api.openai.com") {
		// Parse OpenAI response format
		var openaiResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
			mudlog.Error("LLM", "error", fmt.Sprintf("Failed to decode OpenAI response: %v", err))
			return LLMResponse{Error: fmt.Errorf("failed to decode OpenAI response: %v", err)}
		}

		if openaiResp.Error != nil && openaiResp.Error.Message != "" {
			mudlog.Error("LLM", "error", fmt.Sprintf("OpenAI error: %s", openaiResp.Error.Message))
			return LLMResponse{Error: fmt.Errorf("OpenAI error: %s", openaiResp.Error.Message)}
		}

		if len(openaiResp.Choices) == 0 {
			mudlog.Error("LLM", "error", "No choices in OpenAI response")
			return LLMResponse{Error: fmt.Errorf("no choices in OpenAI response")}
		}

		responseText = openaiResp.Choices[0].Message.Content

		// Get token counts from the response
		inputTokens = openaiResp.Usage.PromptTokens
		outputTokens = openaiResp.Usage.CompletionTokens

		mudlog.Info("LLM", "token_usage", fmt.Sprintf("OpenAI tokens: %d prompt, %d completion, %d total",
			inputTokens, outputTokens, openaiResp.Usage.TotalTokens))
	} else {
		// Parse Ollama response format
		var ollamaResp struct {
			Response string `json:"response"`
			Error    string `json:"error,omitempty"`
			Done     bool   `json:"done"`
			// Some versions of Ollama include token stats
			PromptEvalCount int   `json:"prompt_eval_count,omitempty"`
			EvalCount       int   `json:"eval_count,omitempty"`
			TotalDuration   int64 `json:"total_duration,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			mudlog.Error("LLM", "error", fmt.Sprintf("Failed to decode Ollama response: %v", err))
			return LLMResponse{Error: fmt.Errorf("failed to decode Ollama response: %v", err)}
		}

		if ollamaResp.Error != "" {
			mudlog.Error("LLM", "error", fmt.Sprintf("Ollama error: %s", ollamaResp.Error))
			return LLMResponse{Error: fmt.Errorf("Ollama error: %s", ollamaResp.Error)}
		}

		if !ollamaResp.Done {
			mudlog.Error("LLM", "error", "Response not complete (done=false)")
			return LLMResponse{Error: fmt.Errorf("response not complete")}
		}

		responseText = ollamaResp.Response

		// Get token counts if available, otherwise estimate
		if ollamaResp.PromptEvalCount > 0 || ollamaResp.EvalCount > 0 {
			inputTokens = ollamaResp.PromptEvalCount
			outputTokens = ollamaResp.EvalCount
			mudlog.Info("LLM", "token_usage", fmt.Sprintf("Ollama tokens: %d prompt, %d completion",
				inputTokens, outputTokens))
		} else {
			// Estimate token counts
			inputTokens = EstimateTokenCount(fullPrompt)
			outputTokens = EstimateTokenCount(responseText)
			mudlog.Info("LLM", "token_usage", fmt.Sprintf("Estimated tokens: ~%d prompt, ~%d completion",
				inputTokens, outputTokens))
		}
	}

	mudlog.Info("LLM", "response", fmt.Sprintf("Received response in %v", time.Since(start)))

	// Record token usage if userId provided
	if len(userId) > 0 && userId[0] > 0 {
		RecordTokenUsage(userId[0], string(config.Model), inputTokens, outputTokens)
		mudlog.Info("LLM", "tokens_recorded", fmt.Sprintf("Recorded token usage for user %d", userId[0]))
	}

	return LLMResponse{
		Text:     responseText,
		Duration: time.Since(start),
	}
}

// Returns true if requests are in a penalty box
func isRequestBackoff() bool {
	waitMutex.RLock()
	defer waitMutex.RUnlock()
	return waitUntil.After(time.Now())
}

// Sets a time for requests to resume
func doRequestBackoff() {
	waitMutex.Lock()
	waitUntil = time.Now().Add(RequestFailureBackoffSeconds * time.Second)
	waitMutex.Unlock()
}
