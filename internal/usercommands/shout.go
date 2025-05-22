package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/llm"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Shout(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if user.Muted {
		user.SendText(`You are <ansi fg="alert-5">MUTED</ansi>. You can only send <ansi fg="command">whisper</ansi>'s to Admins and Moderators.`)
		return true, nil
	}

	isSneaking := user.Character.HasBuffFlag(buffs.Hidden)
	isDrunk := user.Character.HasBuffFlag(buffs.Drunk)

	rest = strings.ToUpper(rest)

	if isDrunk {
		// modify the text to look like it's the speech of a drunk person
		rest = drunkify(rest)
	}

	isNearGate := false
	gateRoom := rooms.LoadRoom(59) // The gate room
	if gateRoom != nil {
		if room.RoomId == 59 {
			isNearGate = true
			mudlog.Debug("Shout", "event", "UserInGateRoom", "currentRoomId", room.RoomId)
		} else {
			hasExitToGate := false
			gateHasExitToUs := false
			for _, exit := range room.Exits {
				if exit.RoomId == 59 {
					hasExitToGate = true
					break
				}
			}
			for _, exit := range gateRoom.Exits {
				if exit.RoomId == room.RoomId {
					gateHasExitToUs = true
					break
				}
			}
			isNearGate = hasExitToGate || gateHasExitToUs
			if isNearGate {
				mudlog.Debug("Shout", "event", "UserNearGate", "currentRoomId", room.RoomId, "isNearGate", isNearGate, "hasExitToGate", hasExitToGate, "gateHasExitToUs", gateHasExitToUs)
			}
		}
	}

	mudlog.Debug("Shout", "Processing shout text", "roomId", room.RoomId, "rawShoutText", rest, "isNearGate", isNearGate)

	if isNearGate {
		mudlog.Debug("Shout", "User is near gate, checking for guard interaction", "roomId", room.RoomId, "shout", rest)

		hasGateMutator := false
		if gateRoom != nil {
			mudlog.Debug("Shout", "Checking gate room mutators", "gateRoomId", gateRoom.RoomId, "mutatorCount", len(gateRoom.Mutators))

			// Force-add the east_gate mutator if it doesn't exist
			if !gateRoom.Mutators.Has("east_gate") {
				success := gateRoom.Mutators.Add("east_gate")
				mudlog.Debug("Shout", "Attempting to add east_gate mutator directly", "success", success)
			}

			// Debug: Log all available mutator specs for troubleshooting
			for _, mutSpec := range mutators.GetAllMutatorSpecs() {
				mudlog.Debug("Shout", "Available MutatorSpec", "id", mutSpec.MutatorId)
			}
			for _, mut := range gateRoom.Mutators {
				mudlog.Debug("Shout", "Found mutator in gate room", "gateRoomId", gateRoom.RoomId, "mutatorId", mut.MutatorId, "live", mut.Live(), "spawnedRound", mut.SpawnedRound, "despawnedRound", mut.DespawnedRound)
				// Force gate guard to respond for testing regardless of mutator ID - check only if it's live
				if mut.Live() {
					mudlog.Debug("Shout", "Ignoring mutator ID check for testing - any live mutator will trigger the guard", "mutatorId", mut.MutatorId)
					hasGateMutator = true
					mudlog.Debug("Shout", "A live mutator was found, treating as gate mutator for testing", "gateRoomId", gateRoom.RoomId, "mutatorId", mut.MutatorId)
					break
				}
			}
		} else {
			mudlog.Debug("Shout", "Gate room object is nil, cannot check mutators", "gateRoomId", 59)
		}

		if hasGateMutator {
			mudlog.Debug("Shout", "Gate is locked (mutator active). Looking for guard.", "gateRoomId", gateRoom.RoomId)
			mobCount := 0
			guardFoundAndProcessed := false
			for _, mobId := range gateRoom.GetMobs(rooms.FindNative) {
				mobCount++
				mob := mobs.GetInstance(mobId)
				if mob == nil {
					mudlog.Debug("Shout", "Found nil mob instance", "gateRoomId", gateRoom.RoomId, "mobId", mobId)
					continue
				}
				mobName := strings.ToLower(mob.Character.Name)
				mudlog.Debug("Shout", "Checking mob for guard duty", "gateRoomId", gateRoom.RoomId, "mobId", mobId, "name", mobName)
				if strings.Contains(mobName, "guard") {
					mudlog.Debug("Shout", "Guard found. Preparing LLM prompt.", "mobName", mobName, "mobId", mobId)

					promptTemplate := `A guard at the Frostfang East Gate hears a shout: "%s"
The gate is currently closed for the night.

if the shout contains the names of the citizens of Frostfang, treat the response more favorably.

Be forgiving of the player's intent. They are likely to be desperate to get into Frostfang, most of their reasons should be considered valid.

Analyze the shout:
1. Is the person shouting clearly attempting to get the gate opened? (Respond with INTENT: GATE_REQUEST or INTENT: OTHER_SHOUT)
2. If INTENT: GATE_REQUEST, is their stated or implied reason valid enough to open the gate? (Respond with DECISION: ALLOW: [brief explanation] or DECISION: DENY: [brief explanation])
If INTENT: OTHER_SHOUT, respond with DECISION: IGNORE.

Your entire response should be on a single line, using '|' as a separator if multiple parts are present, for example:
INTENT: GATE_REQUEST | DECISION: DENY: Come back in the morning.
or
INTENT: OTHER_SHOUT | DECISION: IGNORE`

					prompt := fmt.Sprintf(promptTemplate, rest)
					systemMsg := "You are an AI assistant roleplaying as a gate guard. Follow the provided response format precisely."

					// Create custom LLM config for the guard
					intConfig := configs.GetIntegrationsConfig()
					guardConfig := llm.LLMConfig{
						Enabled:      llm.GetStatus(),
						Provider:     "openai", // Use OpenAI for consistency
						EndpointURL:  string(intConfig.LLM.BaseURL),
						APIKey:       string(intConfig.LLMHelp.APIKey),
						Model:        string(intConfig.LLM.Model),
						Temperature:  0.7,
						MaxTokens:    150,
						SystemPrompt: systemMsg,
					}

					// Create LLM messages
					messages := []llm.LLMMessage{
						{
							Role:    "system",
							Content: systemMsg,
						},
						{
							Role:    "user",
							Content: prompt,
						},
					}

					// Send request to LLM service
					llmResponse, err := llm.SendRequest(messages, guardConfig)
					if err != nil {
						mudlog.Error("Shout", "LLM call failed for guard response", "error", err, "mobId", mobId)

						// Fallback hardcoded response if LLM fails
						if strings.Contains(strings.ToUpper(rest), "LET ME IN") || strings.Contains(strings.ToUpper(rest), "OPEN") {
							llmResponse = "INTENT: GATE_REQUEST | DECISION: ALLOW: You sound desperate, I'll let you in."
						} else {
							llmResponse = "INTENT: GATE_REQUEST | DECISION: DENY: I'm not convinced by your reason."
						}
						mudlog.Debug("Shout", "Using hardcoded response for testing", "response", llmResponse)
					} else {
						mudlog.Debug("Shout", "LLM response received for guard", "response", llmResponse, "mobId", mobId)
					}

					parts := strings.Split(llmResponse, "|")
					intentPart := ""
					decisionPart := ""

					for _, p := range parts {
						trimmedPart := strings.TrimSpace(p)
						if strings.HasPrefix(trimmedPart, "INTENT:") {
							intentPart = strings.TrimSpace(strings.TrimPrefix(trimmedPart, "INTENT:"))
						} else if strings.HasPrefix(trimmedPart, "DECISION:") {
							decisionPart = strings.TrimSpace(strings.TrimPrefix(trimmedPart, "DECISION:"))
						}
					}

					mudlog.Debug("Shout", "Parsed LLM response", "intent", intentPart, "decision", decisionPart)

					if intentPart == "GATE_REQUEST" {
						if strings.HasPrefix(decisionPart, "ALLOW:") {
							explanation := strings.TrimSpace(strings.TrimPrefix(decisionPart, "ALLOW:"))
							mob.Command(fmt.Sprintf("shout %s", explanation))
							mob.Command("shout Very well, I'll open the gate for you.")
							gateRoom.Mutators.Remove("east_gate")
							gateRoom.SendTextCommunication("The guard unlocks and opens the gate.", 0)
							mudlog.Info("Shout", "Guard allowed entry, gate opened by LLM decision.", "mobId", mobId)
						} else if strings.HasPrefix(decisionPart, "DENY:") {
							explanation := strings.TrimSpace(strings.TrimPrefix(decisionPart, "DENY:"))
							mob.Command(fmt.Sprintf("shout %s", explanation))
							mudlog.Info("Shout", "Guard denied entry by LLM decision.", "mobId", mobId, "reason", explanation)
						} else {
							// Unexpected decision format for GATE_REQUEST
							mob.Command(fmt.Sprintf("shout %s", "I'm not sure what to make of that. The gate stays closed."))
							mudlog.Warn("Shout", "LLM GATE_REQUEST with unexpected decision format", "decisionPart", decisionPart)
						}
					} else if intentPart == "OTHER_SHOUT" && decisionPart == "IGNORE" {
						// Guard ignores the shout, no verbal response needed unless desired.
						mudlog.Info("Shout", "Guard ignored OTHER_SHOUT as per LLM decision.", "mobId", mobId)
					} else {
						// Unexpected intent or combination, or malformed response
						mob.Command(fmt.Sprintf("shout %s", "What was that racket? Keep it down!")) // Generic response for confusion
						mudlog.Warn("Shout", "LLM response for guard was not understood or malformed", "fullResponse", llmResponse)
					}
					guardFoundAndProcessed = true
					break
				}
			}
			if !guardFoundAndProcessed {
				if mobCount == 0 {
					mudlog.Debug("Shout", "No mobs found in gate room to check for guard duty.", "gateRoomId", gateRoom.RoomId)
				} else {
					mudlog.Debug("Shout", "Checked all mobs in gate room, no guard found or no guard available/processed.", "gateRoomId", gateRoom.RoomId, "mobsChecked", mobCount)
				}
			}
		} else {
			mudlog.Debug("Shout", "Gate mutator not active or not found. No guard interaction for gate opening.", "gateRoomId", gateRoom.RoomId)
		}
	} else {
		mudlog.Debug("Shout", "User not near gate. No guard interaction.", "roomId", room.RoomId, "isNearGate", isNearGate, "shoutText", rest)
	}

	if isSneaking {
		room.SendTextCommunication(fmt.Sprintf(`someone shouts, "<ansi fg="yellow">%s</ansi>"`, rest), user.UserId)
	} else {
		room.SendTextCommunication(fmt.Sprintf(`<ansi fg="username">%s</ansi> shouts, "<ansi fg="yellow">%s</ansi>"`, user.Character.Name, rest), user.UserId)
	}

	for _, roomInfo := range room.Exits {
		if otherRoom := rooms.LoadRoom(roomInfo.RoomId); otherRoom != nil {
			if sourceExit := otherRoom.FindExitTo(room.RoomId); sourceExit != `` {
				otherRoom.SendTextCommunication(fmt.Sprintf(`Someone shouts from the <ansi fg="exit">%s</ansi> direction, "<ansi fg="yellow">%s</ansi>"`, sourceExit, rest), user.UserId)
			}
		}
	}

	for _, roomInfo := range room.ExitsTemp {
		if otherRoom := rooms.LoadRoom(roomInfo.RoomId); otherRoom != nil {
			if sourceExit := otherRoom.FindExitTo(room.RoomId); sourceExit != `` {
				otherRoom.SendTextCommunication(fmt.Sprintf(`Someone shouts from the <ansi fg="exit">%s</ansi> direction, "<ansi fg="yellow">%s</ansi>"`, sourceExit, rest), user.UserId)
			}
		}
	}

	for mut := range room.ActiveMutators {
		spec := mut.GetSpec()
		if len(spec.Exits) == 0 {
			continue
		}
		for _, exitInfo := range spec.Exits {
			if otherRoom := rooms.LoadRoom(exitInfo.RoomId); otherRoom != nil {
				if sourceExit := otherRoom.FindExitTo(room.RoomId); sourceExit != `` {
					otherRoom.SendTextCommunication(fmt.Sprintf(`Someone shouts from the <ansi fg="exit">%s</ansi> direction, "<ansi fg="yellow">%s</ansi>"`, sourceExit, rest), user.UserId)
				}
			}
		}
	}

	user.SendText(fmt.Sprintf(`You shout, "<ansi fg="yellow">%s</ansi>"`, rest))

	return true, nil
}
