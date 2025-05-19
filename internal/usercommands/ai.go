package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// AI handles the command to toggle AI/LLM features for a player
func AI(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if user.Character == nil {
		return false, fmt.Errorf("you don't have a character")
	}

	// Check current setting
	currentSetting := user.Character.GetSetting("llm_disabled")
	newSetting := ""
	message := ""

	// Toggle the setting
	if currentSetting == "true" {
		// Enable LLM features
		newSetting = ""
		message = "<ansi fg=\"green\">AI-powered features enabled!</ansi>\nNPCs can now use LLM responses and the help system will use AI when needed."
	} else {
		// Disable LLM features
		newSetting = "true"
		message = "<ansi fg=\"yellow\">AI-powered features disabled!</ansi>\nNPCs will use only scripted responses and the help system will use only predefined templates."
	}

	// Update the setting
	user.Character.SetSetting("llm_disabled", newSetting)
	mudlog.Info("ai-command", "action", fmt.Sprintf("User %d toggled LLM features to: %s", user.UserId, newSetting))

	// Send response to user
	user.SendText(message)

	return true, nil
}
