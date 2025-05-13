package configs

type Integrations struct {
	Discord IntegrationsDiscord `yaml:"Discord"`
	LLM     IntegrationsLLM     `yaml:"LLM"`
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
		i.LLM.Model = "llama2" // Default model
	}

	if i.LLM.BaseURL == "" {
		i.LLM.BaseURL = "http://localhost:11434" // Default Ollama URL
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
