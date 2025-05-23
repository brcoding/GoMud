package usercommands

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/conversations"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Track last response time for each NPC to prevent spam
var lastResponseTime = make(map[int]time.Time)

const responseCooldown = 5 * time.Second

// findPotentialMatches returns a list of NPCs that could match the given name or nickname
func findPotentialMatches(room *rooms.Room, name string) []*mobs.Mob {
	var matches []*mobs.Mob
	nameLower := strings.ToLower(name)

	// First check for exact name matches
	for _, mobId := range room.GetMobs() {
		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}
		if strings.ToLower(mob.Character.Name) == nameLower {
			matches = append(matches, mob)
		}
	}

	// If no exact matches, check nicknames
	if len(matches) == 0 {
		for _, mobId := range room.GetMobs() {
			mob := mobs.GetInstance(mobId)
			if mob == nil {
				continue
			}

			// Check if any of the mob's nicknames match
			for _, nickname := range mob.Nicknames {
				if strings.ToLower(nickname) == nameLower {
					matches = append(matches, mob)
					break
				}
			}

			// Also do a partial name match for mobs without nicknames
			if len(mob.Nicknames) == 0 {
				mobName := strings.ToLower(mob.Character.Name)
				// Check if name is part of the mob's name
				if strings.Contains(mobName, nameLower) {
					matches = append(matches, mob)
				}
			}
		}
	}

	return matches
}

// isPlayerName checks if the given name matches any player in the room
func isPlayerName(room *rooms.Room, name string) bool {
	nameLower := strings.ToLower(name)
	for _, playerId := range room.GetPlayers() {
		if user := users.GetByUserId(playerId); user != nil {
			if strings.ToLower(user.Character.Name) == nameLower {
				return true
			}
		}
	}
	return false
}

func Say(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if user.Muted {
		user.SendText(`You are <ansi fg="alert-5">MUTED</ansi>. You can only send <ansi fg="command">whisper</ansi>'s to Admins and Moderators.`)
		return true, nil
	}

	isSneaking := user.Character.HasBuffFlag(buffs.Hidden)
	isDrunk := user.Character.HasBuffFlag(buffs.Drunk)

	if isDrunk {
		// modify the text to look like it's the speech of a drunk person
		rest = drunkify(rest)
	}

	// Check if player is in an active conversation with an NPC
	for _, mobId := range room.GetMobs() {
		mob := mobs.GetInstance(mobId)
		if mob == nil || !mob.InConversation() {
			continue
		}

		// Get the conversation ID from the mob
		conversationId := mob.GetConversationId()
		if conversationId <= 0 {
			continue
		}

		// Check if this conversation involves the current player
		conversation := conversations.GetConversation(conversationId)
		if conversation == nil {
			continue
		}

		if conversation.MobInstanceId1 == mob.InstanceId && conversation.MobInstanceId2 == user.UserId {
			// First show the player's message immediately
			if isSneaking {
				room.SendTextCommunication(fmt.Sprintf(`someone says, "<ansi fg="saytext">%s</ansi>"`, rest), user.UserId)
			} else {
				room.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> says, "<ansi fg="saytext">%s</ansi>"`, user.Character.Name, rest), user.UserId)
			}
			user.SendText(fmt.Sprintf(`You say, "<ansi fg="saytext">%s</ansi>"`, rest))

			// Process the NPC's response asynchronously
			go func(convId int, userInput string, mobInst *mobs.Mob, userRef *users.UserRecord, roomRef *rooms.Room) {
				// Player is participant in this conversation
				response, err := conversations.ProcessPlayerInput(convId, userInput)
				if err == nil && response != "" {
					// Send the response to the player
					if !userRef.Character.HasBuffFlag(buffs.Hidden) {
						roomRef.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> says to <ansi fg="username">%s</ansi>, "<ansi fg="saytext">%s</ansi>"`, mobInst.Character.Name, userRef.Character.Name, response), userRef.UserId)
					} else {
						roomRef.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> says to someone, "<ansi fg="saytext">%s</ansi>"`, mobInst.Character.Name, response), userRef.UserId)
					}
					userRef.SendText(fmt.Sprintf(`<ansi fg="username">%s</ansi> says to you, "<ansi fg="saytext">%s</ansi>"`, mobInst.Character.Name, response))

					mudlog.Debug("ProcessPlayerInput", "response", fmt.Sprintf("NPC response: %s", response))
				} else if err != nil {
					mudlog.Error("ProcessPlayerInput", "error", fmt.Sprintf("Error processing player input: %v", err))
				}
			}(conversationId, rest, mob, user, room)

			// Since we've processed the conversation, we can return
			return true, nil
		}
	}

	// Extract potential names from the message
	words := strings.Fields(strings.ToLower(rest))
	var potentialNames []string
	for _, word := range words {
		// Skip common words and short words
		if len(word) < 3 || isCommonWord(word) {
			continue
		}
		potentialNames = append(potentialNames, word)
	}

	// Check each potential name
	for _, name := range potentialNames {
		// If it's a player name, skip it
		if isPlayerName(room, name) {
			continue
		}

		// Find potential NPC matches
		matches := findPotentialMatches(room, name)
		mudlog.Debug("Say", "info", fmt.Sprintf("name: %v room: %v matches: %v", name, room, matches))

		// If we have exactly one match, process it
		if len(matches) == 1 {
			mob := matches[0]
			if mob == nil {
				mudlog.Debug("Say", "error", "Found nil mob in matches")
				continue
			}

			// Check cooldown
			if lastTime, exists := lastResponseTime[mob.InstanceId]; exists {
				if time.Since(lastTime) < responseCooldown {
					mudlog.Debug("Say", "info", fmt.Sprintf("Mob %v is on cooldown", mob.InstanceId))
					continue
				}
			}

			// Check if the mob has a conversation file with LLM enabled
			hasConverseFile := conversations.HasConverseFile(int(mob.MobId), mob.Character.Zone)
			mudlog.Debug("Say", "info", fmt.Sprintf("hasConversationFile: %v mob: %v mobId: %v zone: %v", hasConverseFile, mob, mob.MobId, mob.Character.Zone))
			if hasConverseFile {
				if handleNPCResponse(mob, user, room, rest, strings.ToLower(rest)) {
					lastResponseTime[mob.InstanceId] = time.Now()
					break
				}
			}
		} else if len(matches) > 1 {
			mudlog.Debug("Say", "info", fmt.Sprintf("Multiple matches found for name %v: %v", name, matches))
		}
	}

	if isSneaking {
		room.SendTextCommunication(fmt.Sprintf(`someone says, "<ansi fg="saytext">%s</ansi>"`, rest), user.UserId)
	} else {
		room.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> says, "<ansi fg="saytext">%s</ansi>"`, user.Character.Name, rest), user.UserId)
	}

	user.SendText(fmt.Sprintf(`You say, "<ansi fg="saytext">%s</ansi>"`, rest))

	room.SendTextToExits(`You hear someone talking.`, true)

	events.AddToQueue(events.Communication{
		SourceUserId: user.UserId,
		CommType:     `say`,
		Name:         user.Character.Name,
		Message:      rest,
	})

	return true, nil
}

// isCommonWord checks if a word is too common to be a name
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"the": true, "and": true, "but": true, "for": true, "not": true,
		"you": true, "that": true, "this": true, "with": true, "from": true,
		"hey": true, "hi": true, "hello": true, "greetings": true,
		"please": true, "thank": true, "thanks": true, "sorry": true,
		"excuse": true, "pardon": true, "yes": true, "no": true,
	}
	return commonWords[word]
}

// handleNPCResponse processes the NPC's response to a message
func handleNPCResponse(mob *mobs.Mob, user *users.UserRecord, room *rooms.Room, originalMessage, messageLower string) bool {
	// Build context for the LLM
	context := map[string]interface{}{
		"player_name": user.Character.Name,
		"message":     originalMessage,
		"is_drunk":    user.Character.HasBuffFlag(buffs.Drunk),
		"is_sneaking": user.Character.HasBuffFlag(buffs.Hidden),
		"room_name":   room.Title,
		"zone":        room.Zone,
	}

	// Add any visible items or NPCs in the room for context
	var roomContents []string
	for _, mobId := range room.GetMobs() {
		mudlog.Debug("room.GetMobs()", "info", fmt.Sprintf("mobId: %v", mobId))
		if otherMob := mobs.GetInstance(mobId); otherMob != nil && otherMob.InstanceId != mob.InstanceId {
			roomContents = append(roomContents, otherMob.Character.Name)
		}
	}
	context["room_contents"] = roomContents

	// Start a conversation with the mob
	conversationId := conversations.AttemptConversation(
		int(mob.MobId),
		mob.InstanceId,
		mob.Character.Name,
		user.UserId,
		user.Character.Name,
		mob.Character.Zone,
	)

	if conversationId > 0 {
		mob.SetConversation(conversationId)
		mudlog.Debug("NPC Response", "context", fmt.Sprintf("Context for %s: %v", mob.Character.Name, context))

		// Process the NPC's response asynchronously
		go func(convId int, userInput string, mobInst *mobs.Mob, userRef *users.UserRecord, roomRef *rooms.Room) {
			// Get initial greeting or response from the conversation
			response, err := conversations.ProcessPlayerInput(convId, userInput)
			if err == nil && response != "" {
				// Send the response to the player
				if !userRef.Character.HasBuffFlag(buffs.Hidden) {
					roomRef.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> says to <ansi fg="username">%s</ansi>, "<ansi fg="saytext">%s</ansi>"`, mobInst.Character.Name, userRef.Character.Name, response), userRef.UserId)
				} else {
					roomRef.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> says to someone, "<ansi fg="saytext">%s</ansi>"`, mobInst.Character.Name, response), userRef.UserId)
				}
				userRef.SendText(fmt.Sprintf(`<ansi fg="username">%s</ansi> says to you, "<ansi fg="saytext">%s</ansi>"`, mobInst.Character.Name, response))

				mudlog.Debug("ProcessPlayerInput", "response", fmt.Sprintf("NPC response: %s", response))
			} else if err != nil {
				mudlog.Error("ProcessPlayerInput", "error", fmt.Sprintf("Error getting response: %v", err))
			}
		}(conversationId, originalMessage, mob, user, room)

		return true
	}

	return false
}

func drunkify(sentence string) string {

	var drunkSentence strings.Builder
	isStartOfWord := true
	sentenceLength := len(sentence)
	insertedHiccup := false

	for i, char := range sentence {
		// Randomly decide whether to modify the character
		if util.Rand(10) < 3 || (!insertedHiccup || i == sentenceLength-1) {
			switch char {
			case 's':
				if isStartOfWord {
					drunkSentence.WriteString("sss")
				} else {
					drunkSentence.WriteString("sh")
				}
			case 'S':
				drunkSentence.WriteString("Sh")
			default:
				drunkSentence.WriteRune(char)
			}

			// Insert a hiccup in the middle of the sentence
			if !insertedHiccup && i >= sentenceLength/2 {
				drunkSentence.WriteString(" *hiccup* ")
				insertedHiccup = true
			}
		} else {
			drunkSentence.WriteRune(char)
		}

		// Update isStartOfWord based on spaces and punctuation
		if char == ' ' || char == '.' || char == '!' || char == '?' || char == ',' {
			isStartOfWord = true
		} else {
			isStartOfWord = false
		}
	}

	return drunkSentence.String()
}
