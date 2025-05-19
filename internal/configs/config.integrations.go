package configs

type Integrations struct {
	Discord IntegrationsDiscord `yaml:"Discord"`
	LLM     IntegrationsLLM     `yaml:"LLM"`
	LLMHelp IntegrationsLLMHelp `yaml:"LLMHelp"`
}

type IntegrationsDiscord struct {
	WebhookUrl ConfigSecret `yaml:"WebhookUrl" env:"DISCORD_WEBHOOK_URL"` // Optional Discord URL to post updates to
}

type IntegrationsLLM struct {
	Enabled          ConfigBool   `yaml:"Enabled"`          // Whether LLM integration is enabled
	Provider         ConfigString `yaml:"Provider"`         // The LLM provider (e.g., "ollama")
	Model            ConfigString `yaml:"Model"`            // The model to use (e.g., "llama2")
	BaseURL          ConfigString `yaml:"BaseURL"`          // Base URL for the LLM API
	Temperature      ConfigFloat  `yaml:"Temperature"`      // Temperature for response generation (0.0-1.0)
	MaxContextLength ConfigInt    `yaml:"MaxContextLength"` // Maximum number of conversation turns to include in context
}

type IntegrationsLLMHelp struct {
	Enabled       ConfigBool   `yaml:"Enabled"`       // Whether LLM help system is enabled
	SystemPrompt  ConfigString `yaml:"SystemPrompt"`  // System prompt for the help LLM
	EndpointURL   ConfigString `yaml:"EndpointURL"`   // URL for the LLM API (if different from the main LLM)
	APIKey        ConfigSecret `yaml:"APIKey"`        // API key (if needed)
	Model         ConfigString `yaml:"Model"`         // Model to use (if different from the main LLM)
	TemplatePath  ConfigString `yaml:"TemplatePath"`  // Path to store generated templates
	SaveResponses ConfigBool   `yaml:"SaveResponses"` // Whether to save responses as templates
}

func (i *Integrations) Validate() {
	// Validate Discord settings
	// Ignore Discord

	// Validate LLM settings
	if i.LLM.Temperature < 0.0 {
		i.LLM.Temperature = 0.7 // Default temperature
	} else if i.LLM.Temperature > 1.0 {
		i.LLM.Temperature = 1.0 // Cap at 1.0
	}

	if i.LLM.MaxContextLength < 1 {
		i.LLM.MaxContextLength = 10 // Default context length
	} else if i.LLM.MaxContextLength > 50 {
		i.LLM.MaxContextLength = 50 // Cap at 50 turns
	}

	if i.LLM.Provider == "" {
		i.LLM.Provider = "ollama" // Default provider
	}

	if i.LLM.Model == "" {
		i.LLM.Model = "llama3.3" // Default model
	}

	if i.LLM.BaseURL == "" {
		i.LLM.BaseURL = "http://localhost:11434" // Default Ollama URL
	}

	// Validate LLMHelp settings
	if string(i.LLMHelp.SystemPrompt) == "" {
		i.LLMHelp.SystemPrompt = "You are a helpful MUD game assistant. Provide concise, accurate answers to player questions about game mechanics and commands." // Default system prompt
	}

	if string(i.LLMHelp.TemplatePath) == "" {
		i.LLMHelp.TemplatePath = "templates/help" // Default template path
	}

	// Use main LLM settings as defaults if not specified
	if string(i.LLMHelp.EndpointURL) == "" && i.LLM.Enabled {
		i.LLMHelp.EndpointURL = i.LLM.BaseURL
	}

	if string(i.LLMHelp.Model) == "" && i.LLM.Enabled {
		i.LLMHelp.Model = i.LLM.Model
	}
}

func GetIntegrationsConfig() Integrations {
	configDataLock.RLock()
	defer configDataLock.RUnlock()

	if !configData.validated {
		configData.Validate()
	}
	return configData.Integrations
}
