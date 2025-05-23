package llm

import "github.com/GoMudEngine/GoMud/internal/mudlog"

// SetupLLMSystem initializes all LLM services
func SetupLLMSystem() {
	// Initialize base LLM service
	Initialize()

	// Initialize help-specific settings
	InitHelpLLM()

	mudlog.Info("LLM", "info", "LLM system fully initialized")
}
