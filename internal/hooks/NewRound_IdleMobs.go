// Round ticks for players
package hooks

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/conversations"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/scripting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

//
// Handle mobs that are bored
//

func IdleMobs(e events.Event) events.ListenerReturn {

	mobPathAnnounce := false // useful for debugging purposes.

	mc := configs.GetMemoryConfig()

	maxBoredom := uint8(mc.MaxMobBoredom)

	allMobInstances := mobs.GetAllMobInstanceIds()

	allowedUnloadCt := len(allMobInstances) - int(mc.MobUnloadThreshold)
	if allowedUnloadCt < 0 {
		allowedUnloadCt = 0
	}

	// Handle idle mob behavior
	tStart := time.Now()
	for _, mobId := range allMobInstances {

		mob := mobs.GetInstance(mobId)
		if mob == nil {
			allowedUnloadCt--
			continue
		}

		if allowedUnloadCt > 0 && mob.BoredomCounter >= maxBoredom {

			if mob.Despawns() {
				mob.Command(`despawn` + fmt.Sprintf(` depression %d/%d`, mob.BoredomCounter, maxBoredom))
				allowedUnloadCt--

			} else {
				mob.BoredomCounter = 0
			}

			continue
		}

		// If idle prevented, it's a one round interrupt (until another comes along)
		if mob.PreventIdle {
			mob.PreventIdle = false
			continue
		}

		// If they are doing some sort of combat thing,
		// Don't do idle actions
		if mob.Character.Aggro != nil {
			if mob.Character.Aggro.UserId > 0 {
				user := users.GetByUserId(mob.Character.Aggro.UserId)
				if user == nil || user.Character.RoomId != mob.Character.RoomId {
					mob.Command(`emote mumbles about losing their quarry.`)
					mob.Character.Aggro = nil
				}
			}
			continue
		}

		if mob.InConversation() {
			// mudlog.Debug("IdleMobs", "info", fmt.Sprintf("mob %s (#%d) inConversation with ID: %d", mob.GetName(), mob.GetInstanceId(), mob.GetConversationId()))
			convId := mob.GetConversationId()
			if convId == 0 {
				mudlog.Error("IdleMobs", "error", fmt.Sprintf("Mob %s (#%d) InConversation() is true but GetConversationId() is 0", mob.GetName(), mob.GetInstanceId()))
				continue
			}

			// Get conversation to check participant types
			conv := conversations.GetConversation(convId)
			if conv == nil {
				mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Conversation %d not found", convId))
				conversations.Destroy(convId)
				continue
			}

			// Check if participants are in the same room
			var mob1RoomId, mob2RoomId int
			if !conv.IsPlayer1 {
				if mob1 := mobs.GetInstance(conv.MobInstanceId1); mob1 != nil {
					mob1RoomId = mob1.Character.RoomId
				}
			} else {
				if user := users.GetByUserId(conv.MobInstanceId1); user != nil {
					mob1RoomId = user.Character.RoomId
				}
			}

			if !conv.IsPlayer2 {
				if mob2 := mobs.GetInstance(conv.MobInstanceId2); mob2 != nil {
					mob2RoomId = mob2.Character.RoomId
				}
			} else {
				if user := users.GetByUserId(conv.MobInstanceId2); user != nil {
					mob2RoomId = user.Character.RoomId
				}
			}

			// End conversation if participants are in different rooms or either participant is not found
			if mob1RoomId == 0 || mob2RoomId == 0 || mob1RoomId != mob2RoomId {

				// Generate a farewell message if they were in the same room before
				if mob1RoomId != 0 && mob2RoomId != 0 && mob1RoomId != mob2RoomId {
					if !conv.IsPlayer1 {
						if mob1 := mobs.GetInstance(conv.MobInstanceId1); mob1 != nil {
							// Get farewell from conversation system
							farewell, err := conversations.EndConversation(convId)
							if err == nil && farewell != "" {
								// Send the farewell message
								room := rooms.LoadRoom(mob1.Character.RoomId)
								if room != nil {
									room.SendText(fmt.Sprintf(`<ansi fg="username">%s</ansi> says, "<ansi fg="saytext">%s</ansi>"`, mob1.Character.Name, farewell))
								}
							}
						}
					}
				}

				conversations.Destroy(convId)

				// Clear conversation IDs from participants
				if !conv.IsPlayer1 {
					if mob1 := mobs.GetInstance(conv.MobInstanceId1); mob1 != nil {
						mob1.SetConversation(0)
					}
				}
				if !conv.IsPlayer2 {
					if mob2 := mobs.GetInstance(conv.MobInstanceId2); mob2 != nil {
						mob2.SetConversation(0)
					}
				}

				continue
			}

			// GetNextActions now returns concrete mob instance IDs
			mob1InstId, mob2InstId, actions := conversations.GetNextActions(convId)

			if len(actions) > 0 {
				// Get participants based on their type
				var mob1, mob2 *mobs.Mob
				if !conv.IsPlayer1 {
					mob1 = mobs.GetInstance(mob1InstId)
				}
				if !conv.IsPlayer2 {
					mob2 = mobs.GetInstance(mob2InstId)
				}

				// Validate mobs if they are mobs
				if !conv.IsPlayer1 && mob1 == nil {
					mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Invalid mob1 instance during conversation %d (ID: %d). Destroying conversation.", convId, mob1InstId))
					conversations.Destroy(convId)
					continue
				}
				if !conv.IsPlayer2 && mob2 == nil {
					mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Invalid mob2 instance during conversation %d (ID: %d). Destroying conversation.", convId, mob2InstId))
					conversations.Destroy(convId)
					continue
				}

				// Validate players if they are players
				if conv.IsPlayer1 {
					if user := users.GetByUserId(mob1InstId); user == nil {
						mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Player1 no longer valid during conversation %d (ID: %d). Destroying conversation.", convId, mob1InstId))
						conversations.Destroy(convId)
						continue
					}
				}
				if conv.IsPlayer2 {
					if user := users.GetByUserId(mob2InstId); user == nil {
						mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Player2 no longer valid during conversation %d (ID: %d). Destroying conversation.", convId, mob2InstId))
						conversations.Destroy(convId)
						continue
					}
				}

				for _, act := range actions {
					if len(act) >= 3 {
						targetPrefix := act[0:3]
						cmd := act[3:]

						// Skip placeholder actions
						if cmd == "*" {
							continue
						}

						// Replace placeholders with appropriate IDs
						if !conv.IsPlayer1 && mob1 != nil {
							cmd = strings.ReplaceAll(cmd, ` #1 `, ` `+mob1.ShorthandId()+` `)
						}
						if !conv.IsPlayer2 && mob2 != nil {
							cmd = strings.ReplaceAll(cmd, ` #2 `, ` `+mob2.ShorthandId()+` `)
						}

						// Get the room for sending messages
						var room *rooms.Room
						if !conv.IsPlayer1 && mob1 != nil {
							room = rooms.LoadRoom(mob1.Character.RoomId)
						} else if conv.IsPlayer1 {
							if user := users.GetByUserId(mob1InstId); user != nil {
								room = rooms.LoadRoom(user.Character.RoomId)
							}
						}
						if room == nil {
							mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Could not find room for conversation %d", convId))
							continue
						}

						if targetPrefix == `#1 ` {
							if !conv.IsPlayer1 && mob1 != nil {
								// Execute the command - the event system will handle room output
								if strings.HasPrefix(cmd, "sayto") {
									// For sayto commands, we need to ensure the target is properly set
									parts := strings.SplitN(cmd, " ", 3)
									if len(parts) >= 3 {
										// Get the target based on conversation participant
										var targetName string
										if conv.IsPlayer2 {
											// If player2 is the target, use their name
											if user := users.GetByUserId(mob2InstId); user != nil {
												targetName = user.Character.Name
											}
										} else if mob2 != nil {
											// If mob2 is the target, use their name
											targetName = mob2.Character.Name
										}

										if targetName != "" {
											// Reconstruct the command with proper target name
											newCmd := fmt.Sprintf("sayto %s %s", targetName, parts[2])
											mob1.Command(newCmd)
										} else {
											mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Could not resolve target for sayto command in conversation %d", convId))
										}
									}
								} else {
									mob1.Command(cmd)
								}
							} else if conv.IsPlayer1 {
								// Player1 speaking
								if user := users.GetByUserId(mob1InstId); user != nil {
									user.Command(cmd)
								}
							}
						} else if targetPrefix == `#2 ` {
							if !conv.IsPlayer2 && mob2 != nil {
								// Execute the command - the event system will handle room output
								if strings.HasPrefix(cmd, "sayto") {
									// For sayto commands, we need to ensure the target is properly set
									parts := strings.SplitN(cmd, " ", 3)
									if len(parts) >= 3 {
										// Get the target based on conversation participant
										var targetName string
										if conv.IsPlayer1 {
											// If player1 is the target, use their name
											if user := users.GetByUserId(mob1InstId); user != nil {
												targetName = user.Character.Name
											}
										} else if mob1 != nil {
											// If mob1 is the target, use their name
											targetName = mob1.Character.Name
										}

										if targetName != "" {
											// Reconstruct the command with proper target name
											newCmd := fmt.Sprintf("sayto %s %s", targetName, parts[2])
											mob2.Command(newCmd, 1)
										} else {
											mudlog.Error("IdleMobs", "conversation_error", fmt.Sprintf("Could not resolve target for sayto command in conversation %d", convId))
										}
									}
								} else {
									mob2.Command(cmd, 1)
								}
							} else if conv.IsPlayer2 {
								// Player2 speaking
								if user := users.GetByUserId(mob2InstId); user != nil {
									user.Command(cmd)
								}
							}
						} else {
							mudlog.Error("IdleMobs", "conversation_action_error", fmt.Sprintf("Unknown target prefix '%s' in action '%s' for conversation %d", targetPrefix, act, convId))
						}
					}
				}

				// After executing actions, check if conversation is complete
				if conversations.IsComplete(convId) {
					// Explicitly destroy the conversation to ensure cleanup
					conversations.Destroy(convId)
					// Clear conversation IDs from participants and unblock player input
					if !conv.IsPlayer1 {
						if mob1 := mobs.GetInstance(mob1InstId); mob1 != nil {
							mob1.SetConversation(0)
						}
					} else {
						// Unblock player input if they were in the conversation
						if user := users.GetByUserId(mob1InstId); user != nil {
							user.UnblockInput()
						}
					}
					if !conv.IsPlayer2 {
						if mob2 := mobs.GetInstance(mob2InstId); mob2 != nil {
							mob2.SetConversation(0)
						}
					} else {
						// Unblock player input if they were in the conversation
						if user := users.GetByUserId(mob2InstId); user != nil {
							user.UnblockInput()
						}
					}
					continue // Skip the IsComplete check below since we just handled it
				}
			}

			// Only check IsComplete if we haven't already destroyed the conversation
			// if !conversations.IsComplete(convId) {
			// 	mudlog.Debug("IdleMobs", "info", fmt.Sprintf("Conversation %d still active for mob %s (#%d)", convId, mob.GetName(), mob.GetInstanceId()))
			// }
			continue // Processed conversation, skip other idle actions for this mob
		}

		// Check whether they are currently in the middle of a path, or have one waiting to start.
		// This comes after checks for whether they are currently in a conersation, or in combat, etc.
		if currentStep := mob.Path.Current(); currentStep != nil || mob.Path.Len() > 0 {

			if currentStep == nil {

				if endPathingAndSkip, _ := scripting.TryMobScriptEvent("onPath", mob.InstanceId, 0, ``, map[string]any{`status`: `start`}); endPathingAndSkip {
					mob.Path.Clear()
					continue
				}

				if mobPathAnnounce {
					mob.Command(`say I'm beginning a new path.`)
				}
			} else {

				// If their currentStep isn't actually the room they are in
				// They've somehow been moved. Reclaculate a new path.
				if currentStep.RoomId() != mob.Character.RoomId {
					if mobPathAnnounce {
						mob.Command(`say I seem to have wandered off my path.`)
					}

					reDoWaypoints := mob.Path.Waypoints()
					if len(reDoWaypoints) > 0 {
						newCommand := `pathto`
						for _, wpInt := range reDoWaypoints {
							newCommand += ` ` + strconv.Itoa(wpInt)
						}
						mob.Command(newCommand)
						continue
					}

					// if we were unable to come up with a new path, send them home.
					mob.Command(`pathto home`)

					continue
				}

				if currentStep.Waypoint() {
					if mobPathAnnounce {
						mob.Command(`say I've reached a waypoint.`)
					}
				}
			}

			if nextStep := mob.Path.Next(); nextStep != nil {

				if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
					if exitInfo, ok := room.Exits[nextStep.ExitName()]; ok {
						if exitInfo.RoomId == nextStep.RoomId() {
							mob.Command(nextStep.ExitName())
							continue
						}
					}
				}

			}

			mob.Path.Clear()

			if endPathingAndSkip, _ := scripting.TryMobScriptEvent("onPath", mob.InstanceId, 0, ``, map[string]any{`status`: `end`}); endPathingAndSkip {
				continue
			}

			if mobPathAnnounce {
				mob.Command(`say I'm.... done.`)
			}

		}

		events.AddToQueue(events.MobIdle{MobInstanceId: mobId})

	}

	util.TrackTime(`IdleMobs()`, time.Since(tStart).Seconds())

	return events.Continue
}
