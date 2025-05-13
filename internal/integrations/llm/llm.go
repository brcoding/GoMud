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
func GenerateResponse(prompt string, context []string) LLMResponse {
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

	// Prepare the request payload according to Ollama's API
	payload := map[string]interface{}{
		"model":       string(config.Model),
		"prompt":      fullPrompt,
		"temperature": float64(config.Temperature),
		"stream":      false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		mudlog.Error("LLM", "error", fmt.Sprintf("Failed to marshal request: %v", err))
		return LLMResponse{Error: fmt.Errorf("failed to marshal request: %v", err)}
	}

	// Send the request
	url := fmt.Sprintf("%s/api/generate", string(config.BaseURL))
	mudlog.Debug("LLM", "request", fmt.Sprintf("Sending request to %s", url))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		mudlog.Error("LLM", "error", fmt.Sprintf("Failed to create request: %v", err))
		return LLMResponse{Error: fmt.Errorf("failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")

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

	// Parse the response according to Ollama's API format
	var result struct {
		Response string `json:"response"`
		Error    string `json:"error,omitempty"`
		Done     bool   `json:"done"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		mudlog.Error("LLM", "error", fmt.Sprintf("Failed to decode response: %v", err))
		return LLMResponse{Error: fmt.Errorf("failed to decode response: %v", err)}
	}

	if result.Error != "" {
		mudlog.Error("LLM", "error", fmt.Sprintf("LLM error: %s", result.Error))
		return LLMResponse{Error: fmt.Errorf("LLM error: %s", result.Error)}
	}

	if !result.Done {
		mudlog.Error("LLM", "error", "Response not complete (done=false)")
		return LLMResponse{Error: fmt.Errorf("response not complete")}
	}

	mudlog.Info("LLM", "response", fmt.Sprintf("Received response in %v", time.Since(start)))

	return LLMResponse{
		Text:     result.Response,
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
