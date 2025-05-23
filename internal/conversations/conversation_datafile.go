package conversations

// LLMConversationConfig defines LLM-specific settings for a conversation
type LLMConversationConfig struct {
	// Whether to use LLM for dynamic responses
	Enabled bool `yaml:"enabled"`
	// System prompt to guide the LLM's responses
	SystemPrompt string `yaml:"systemprompt"`
	// Maximum number of conversation turns to keep in context
	MaxContextTurns int `yaml:"maxcontextturns"`
	// Whether to include NPC names in the context
	IncludeNames bool `yaml:"includenames"`
	// Optional initial greeting message
	Greeting string `yaml:"greeting,omitempty"`
	// Optional farewell message
	Farewell string `yaml:"farewell,omitempty"`
	// Time in seconds before conversation times out
	IdleTimeout int `yaml:"idletimeout,omitempty"`
}

type ConversationData struct {
	// A map of lowercase names of "Initiator" (#1) to array of
	// "Participant" (#2) names allowed to use this conversation.
	Supported map[string][]string `yaml:"supported"`
	// Optional LLM configuration for this conversation
	LLMConfig *LLMConversationConfig `yaml:"llmconfig"`
}
