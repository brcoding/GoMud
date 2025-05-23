package llm

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Configuration for the LLM-based help system
type HelpLLMConfig struct {
	Enabled       bool   `yaml:"enabled"`
	SystemPrompt  string `yaml:"systemprompt"`
	EndpointURL   string `yaml:"endpoint_url"`
	APIKey        string `yaml:"api_key"`
	Model         string `yaml:"model"`
	TemplatePath  string `yaml:"template_path"`
	SaveResponses bool   `yaml:"save_responses"`
}

var helpLLMConfig HelpLLMConfig

// InitHelpLLM initializes the LLM-based help system
func InitHelpLLM() {
	// Load config from the main config or use defaults
	intConfig := configs.GetIntegrationsConfig()

	helpLLMConfig = HelpLLMConfig{
		Enabled:       bool(intConfig.LLMHelp.Enabled),
		SystemPrompt:  string(intConfig.LLMHelp.SystemPrompt),
		EndpointURL:   string(intConfig.LLMHelp.EndpointURL),
		APIKey:        string(intConfig.LLMHelp.APIKey),
		Model:         string(intConfig.LLMHelp.Model),
		TemplatePath:  string(intConfig.LLMHelp.TemplatePath),
		SaveResponses: bool(intConfig.LLMHelp.SaveResponses),
	}

	// Use defaults if any values are empty
	if helpLLMConfig.SystemPrompt == "" {
		helpLLMConfig.SystemPrompt = "You are a helpful MUD game assistant. Provide concise, accurate answers to player questions about game mechanics and commands. Keep responses under 500 words and focus on giving practical, accurate information. IMPORTANT: Only suggest commands that appear in the list of available commands that will be provided to you. Never suggest commands that aren't in this list. Always verify a command exists before suggesting players can use it.\n\nFORMATTING INSTRUCTIONS:\n1. Format all command names using <ansi fg=\"command\">commandname</ansi>\n2. Format skill names using <ansi fg=\"skill\">skillname</ansi>\n3. Format usernames using <ansi fg=\"username\">username</ansi>\n4. Format item names using <ansi fg=\"item\">itemname</ansi>\n5. Use <ansi fg=\"yellow\">Usage: </ansi> before showing command usage examples\n6. Begin all help responses with <ansi fg=\"black-bold\">.:</ansi> <ansi fg=\"magenta\">Help for </ansi><ansi fg=\"command\">topic</ansi>\n7. Format section headings with <ansi fg=\"magenta-bold\">Section Heading:</ansi>\n8. Format example commands to be on their own line indented with two spaces\n\nThis formatting ensures your response matches the style of the game's built-in help system."
	}

	if helpLLMConfig.TemplatePath == "" {
		helpLLMConfig.TemplatePath = "templates/help"
	}

	// Also initialize the shared LLM config
	globalConfig.SystemPrompt = helpLLMConfig.SystemPrompt
}

// saveHelpResponse saves the LLM's response as a template file
func saveHelpResponse(query string, response string) error {
	// Only save templates for single-word queries
	if strings.Contains(strings.TrimSpace(query), " ") {
		mudlog.Info("llm-help", "not_saving", "Query contains spaces, not saving as template", "query", query)
		return nil
	}

	// Create a sanitized filename from the query
	filename := sanitizeFilename(query)
	if filename == "" {
		return fmt.Errorf("could not create valid filename from query")
	}

	// Ensure the directory exists
	templateDir := filepath.Join(helpLLMConfig.TemplatePath, "auto")
	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		mudlog.Error("llm-help", "error", "Failed to create template directory", "path", templateDir, "err", err)
		return err
	}

	// Write the response to the file
	templatePath := filepath.Join(templateDir, filename+".md")
	err = ioutil.WriteFile(templatePath, []byte(response), 0644)
	if err != nil {
		mudlog.Error("llm-help", "error", "Failed to save template file", "path", templatePath, "err", err)
		return err
	}

	mudlog.Info("llm-help", "action", "Saved response template", "path", templatePath)
	return nil
}

// sanitizeFilename creates a valid filename from a query string
func sanitizeFilename(query string) string {
	// Convert to lowercase
	name := strings.ToLower(query)

	// Replace spaces with hyphens
	name = strings.ReplaceAll(name, " ", "-")

	// Remove any characters that aren't alphanumeric, hyphens, or underscores
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, name)

	// Limit the length
	if len(name) > 50 {
		name = name[:50]
	}

	// Ensure it's not empty
	if name == "" {
		name = "unknown-query"
	}

	return name
}

// IsLLMDisabledForPlayer checks if the player has disabled LLM features for themselves
func IsLLMDisabledForPlayer(userId int) bool {
	// Get the player's character
	player := users.GetByUserId(userId)
	if player == nil || player.Character == nil {
		return false // Default to enabled if can't find character
	}

	// Check if the player has disabled LLM features
	return player.Character.GetSetting("llm_disabled") == "true"
}

// IsHelpTemplateAvailable checks if a help template exists for the given query
func IsHelpTemplateAvailable(query string) bool {
	// First, check if the template exists in the standard location
	if templates.Exists("help/" + query) {
		return true
	}

	// Then check if it exists in the auto-generated location
	filename := sanitizeFilename(query)
	return templates.Exists("help/auto/" + filename)
}

// GetAutoGeneratedTemplateName returns the template name for an auto-generated help topic
func GetAutoGeneratedTemplateName(query string) string {
	return "help/auto/" + sanitizeFilename(query)
}
