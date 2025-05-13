package conversations

// LLMConversationConfig defines LLM-specific settings for a conversation
type LLMConversationConfig struct {
	// Whether to use LLM for dynamic responses
	Enabled bool `yaml:"Enabled"`
	// System prompt to guide the LLM's responses
	SystemPrompt string `yaml:"SystemPrompt"`
	// Maximum number of conversation turns to keep in context
	MaxContextTurns int `yaml:"MaxContextTurns"`
	// Whether to include NPC names in the context
	IncludeNames bool `yaml:"IncludeNames"`
}

type ConversationData struct {
	// A map of lowercase names of "Initiator" (#1) to array of
	// "Participant" (#2) names allowed to use this conversation.
	Supported map[string][]string `yaml:"Supported"`
	// A list of command lists, each prefixed with which Mob should execute
	// the action (#1 or #2)
	Conversation [][]string `yaml:"Conversation"`
	// Optional LLM configuration for this conversation
	LLMConfig *LLMConversationConfig `yaml:"LLMConfig,omitempty"`
}
