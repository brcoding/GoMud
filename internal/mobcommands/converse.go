package mobcommands

import (
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/conversations"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Converse(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	// mudlog.Debug("Converse", "info", fmt.Sprintf("main converse method: %v", mob))
	// Don't bother if no players are present
	if mob.InConversation() {
		return true, nil
	}

	if !conversations.HasConverseFile(int(mob.MobId), mob.Character.Zone) {
		return true, nil
	}

	isSneaking := mob.Character.HasBuffFlag(buffs.Hidden)

	if isSneaking {
		return true, nil
	}

	for _, mobInstId := range room.GetMobs() {

		if mobInstId == mob.InstanceId { // no conversing with self
			continue
		}

		if m := mobs.GetInstance(mobInstId); m != nil {
			// mudlog.Debug("Converse", "info", fmt.Sprintf("m: %v", m))
			// Not allowed to start another conversation until this one concludes
			if m.InConversation() {
				continue
			}

			conversationId := 0
			if rest != `` {
				// mudlog.Debug("Converse", "info", fmt.Sprintf("rest: %v", rest))
				forceIndex, _ := strconv.Atoi(rest)
				conversationId = conversations.AttemptConversation(int(mob.MobId), mob.InstanceId, mob.Character.Name,
					m.InstanceId, m.Character.Name,
					mob.Character.Zone, forceIndex)
			} else {
				// mudlog.Debug("Converse", "info", fmt.Sprintf("else rest: %v", rest))
				conversationId = conversations.AttemptConversation(int(mob.MobId), mob.InstanceId, mob.Character.Name,
					m.InstanceId, m.Character.Name,
					mob.Character.Zone)
			}

			if conversationId > 0 {
				// mudlog.Debug("Converse", "info", fmt.Sprintf("conversationId: %v", conversationId))
				mob.SetConversation(conversationId)
				m.SetConversation(conversationId)
				return true, nil
			}
		}
	}

	// If no mob conversation partner found, try with players
	for _, playerId := range room.GetPlayers() {
		user := users.GetByUserId(playerId)
		if user == nil {
			continue
		}
		if user.Character.HasBuffFlag(buffs.Hidden) {
			continue // don't converse with sneaking players
		}
		// Optionally: if you track player conversations, skip if already in one
		// if user.InConversation() { continue }

		conversationId := 0
		if rest != `` {
			// mudlog.Debug("Converse", "info", fmt.Sprintf("rest (player): %v", rest))
			forceIndex, _ := strconv.Atoi(rest)
			conversationId = conversations.AttemptConversation(int(mob.MobId), mob.InstanceId, mob.Character.Name,
				user.UserId, user.Character.Name,
				mob.Character.Zone, forceIndex)
		} else {
			// mudlog.Debug("Converse", "info", fmt.Sprintf("else rest (player): %v", rest))
			conversationId = conversations.AttemptConversation(int(mob.MobId), mob.InstanceId, mob.Character.Name,
				user.UserId, user.Character.Name,
				mob.Character.Zone)
		}

		if conversationId > 0 {
			// mudlog.Debug("Converse", "info", fmt.Sprintf("conversationId (player): %v", conversationId))
			mob.SetConversation(conversationId)
			// Optionally: set player as in conversation if you track that
			return true, nil
		}
	}

	return true, nil
}
