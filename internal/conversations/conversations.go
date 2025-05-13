package conversations

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/integrations/llm"
	"github.com/GoMudEngine/GoMud/internal/mobinterfaces"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

var (
	converseCheckCache   = map[string]bool{}
	conversations        = map[int]*Conversation{}
	conversationCounter  = map[string]int{}
	conversationUniqueId = 0
	conversationMutex    sync.RWMutex  // Mutex for conversations map
	shutdownChan         chan struct{} // Channel to signal shutdown
	shutdownOnce         sync.Once     // Ensure shutdown is called only once
)

// Init initializes the conversations package
func Init() {
	shutdownChan = make(chan struct{})
}

// Shutdown gracefully shuts down the conversations package
func Shutdown() {
	shutdownOnce.Do(func() {
		close(shutdownChan)

		// First, get a list of all conversation IDs to destroy
		conversationMutex.Lock()
		idsToDestroy := make([]int, 0, len(conversations))
		for id := range conversations {
			idsToDestroy = append(idsToDestroy, id)
		}
		conversationMutex.Unlock()

		// Then destroy each conversation without holding the mutex
		for _, id := range idsToDestroy {
			destroyConversation(id)
		}
	})
}

// destroyConversation is the internal version that doesn't acquire the mutex
func destroyConversation(conversationId int) {
	if conv, ok := conversations[conversationId]; ok {
		// Only clear conversation IDs from mob instances, not from players
		if !conv.IsPlayer1 {
			if mob1 := mobinterfaces.GetInstance(conv.MobInstanceId1); mob1 != nil {
				mob1.SetConversation(0)
			}
		}
		if !conv.IsPlayer2 {
			if mob2 := mobinterfaces.GetInstance(conv.MobInstanceId2); mob2 != nil {
				mob2.SetConversation(0)
			}
		}
		conversationMutex.Lock()
		delete(conversations, conversationId)
		conversationMutex.Unlock()
		mudlog.Debug("Conversation", "cleanup", fmt.Sprintf("Destroyed conversation %d", conversationId))
	}
}

// Destroy is the public version that safely destroys a conversation
func Destroy(conversationId int) {
	select {
	case <-shutdownChan:
		// During shutdown, just call destroyConversation directly
		destroyConversation(conversationId)
	default:
		// During normal operation, use a goroutine but check shutdown
		go func() {
			select {
			case <-shutdownChan:
				return
			default:
				destroyConversation(conversationId)
			}
		}()
	}
}

// Returns a non empty ConversationId if successful
func AttemptConversation(initiatorMobId int, initatorInstanceId int, initiatorName string, participantInstanceId int, participantName string, zone string, forceIndex ...int) int {
	mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("initiatorMobId: %v, initatorInstanceId: %v, initiatorName: %v, participantInstanceId: %v, participantName: %v, zone: %v, forceIndex: %v",
		initiatorMobId, initatorInstanceId, initiatorName, participantInstanceId, participantName, zone, forceIndex))
	conversationMutex.Lock()
	defer conversationMutex.Unlock()

	// Check if initiator is a mob
	mob1Interface := mobinterfaces.GetInstance(initatorInstanceId)
	isPlayer1 := false
	if mob1Interface == nil {
		isPlayer1 = true
		mudlog.Debug("AttemptConversation", "info", fmt.Sprintf("Initiator %s (ID: %d) is a player", initiatorName, initatorInstanceId))
	} else {
		mudlog.Debug("AttemptConversation", "info", fmt.Sprintf("Initiator %s (ID: %d) is a mob", initiatorName, initatorInstanceId))
	}

	// First check if participant is a player
	isPlayer2 := false
	if user := users.GetByUserId(participantInstanceId); user != nil {
		isPlayer2 = true
		mudlog.Debug("AttemptConversation", "info", fmt.Sprintf("Participant %s (ID: %d) is a player", participantName, participantInstanceId))
	} else {
		// If not a player, check if it's a mob
		mob2Interface := mobinterfaces.GetInstance(participantInstanceId)
		if mob2Interface == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Invalid participant ID %d - neither a mob nor a player", participantInstanceId))
			return 0
		}
		mudlog.Debug("AttemptConversation", "info", fmt.Sprintf("Participant %s (ID: %d) is a mob", participantName, participantInstanceId))
	}

	// At least one participant must be a mob
	if isPlayer1 && isPlayer2 {
		mudlog.Error("Conversation", "error", "Cannot start conversation: both participants are players")
		return 0
	}

	// Get the actual names from mob instances if they are mobs
	var mob1Name, mob2Name string
	if !isPlayer1 {
		if mob1Interface == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob1 instance not found for ID %d", initatorInstanceId))
			return 0
		}
		mob1Name = mob1Interface.GetName()
		if mob1Name == "" {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob1 has no name (ID: %d)", initatorInstanceId))
			return 0
		}
	} else {
		mob1Name = strings.ToLower(initiatorName)
	}

	if !isPlayer2 {
		mob2Interface := mobinterfaces.GetInstance(participantInstanceId)
		if mob2Interface == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob2 instance not found for ID %d", participantInstanceId))
			return 0
		}
		mob2Name = mob2Interface.GetName()
		if mob2Name == "" {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob2 has no name (ID: %d)", participantInstanceId))
			return 0
		}
	} else {
		mob2Name = strings.ToLower(participantName)
	}

	mudlog.Debug("AttemptConversation", "info", fmt.Sprintf("Participant types - Mob1: %v (name: %s), Mob2: %v (name: %s)",
		!isPlayer1, mob1Name, !isPlayer2, mob2Name))

	zone = ZoneNameSanitize(zone)

	convFolder := string(configs.GetFilePathsConfig().DataFiles) + `/conversations`

	fileName := fmt.Sprintf("%s/%d.yaml", zone, initiatorMobId)

	filePath := util.FilePath(convFolder + `/` + fileName)

	_, err := os.Stat(filePath)
	if err != nil {
		return 0
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		mudlog.Error("AttemptConversation()", "error", "Problem reading conversation datafile "+filePath+": "+err.Error())
		return 0
	}

	var dataFile []ConversationData

	err = yaml.Unmarshal(bytes, &dataFile)
	if err != nil {
		mudlog.Error("AttemptConversation()", "error", "Problem unmarshalling conversation datafile "+filePath+": "+err.Error())
		return 0
	}

	mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("dataFile: %v", dataFile))

	// Actual chosen conversation index
	chosenIndex := 0

	if len(forceIndex) > 0 && forceIndex[0] >= 0 && forceIndex[0] < len(dataFile) {
		chosenIndex = forceIndex[0]
	} else {
		mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("no forceIndex, so finding possible conversations"))
		possibleConversations := []int{}
		for i, content := range dataFile {
			mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("content: %v", content))
			// supportedNameList := content.Supported[initiatorName]
			// if supportedNameList2, ok2 := content.Supported[`*`]; ok2 {
			// 	supportedNameList = append(supportedNameList, supportedNameList2...)
			// }

			// if len(supportedNameList) > 0 {
			// for _, supportedName := range supportedNameList {
			// mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("supportedName: %v", supportedName))
			//if supportedName == participantName || supportedName == `*` {
			possibleConversations = append(possibleConversations, i)
			//}
			// }
			// }
		}
		mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("possibleConversations: %v", possibleConversations))

		if len(possibleConversations) == 0 {
			return 0
		}

		lowestCount := -1
		for _, index := range possibleConversations {
			val := conversationCounter[fmt.Sprintf(`%s:%d`, fileName, index)]
			mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("val: %v", val))
			if val < lowestCount || lowestCount == -1 {
				lowestCount = val
				chosenIndex = index
			}
		}
	}

	trackingTag := fmt.Sprintf(`%s:%d`, fileName, chosenIndex)
	mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("trackingTag: %v", trackingTag))
	conversationCounter[trackingTag] = conversationCounter[trackingTag] + 1
	mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("conversationCounter: %v", conversationCounter))

	conversationUniqueId++

	conversations[conversationUniqueId] = &Conversation{
		Id:             conversationUniqueId,
		MobInstanceId1: initatorInstanceId,
		MobInstanceId2: participantInstanceId,
		PlayerName1:    mob1Name, // Store the actual name, whether it's a mob or player
		PlayerName2:    mob2Name, // Store the actual name, whether it's a mob or player
		IsPlayer1:      isPlayer1,
		IsPlayer2:      isPlayer2,
		StartRound:     util.GetRoundCount(),
		Position:       0,
		ActionList:     dataFile[chosenIndex].Conversation,
		LLMConfig:      dataFile[chosenIndex].LLMConfig,
		Context:        make([]string, 0),
		LLMCooldown:    2 * time.Second,
	}

	mudlog.Debug("AttemptConversation()", "info", fmt.Sprintf("Created conversation: %+v", conversations[conversationUniqueId]))

	return conversationUniqueId
}

func IsComplete(conversationId int) bool {
	conversationMutex.RLock()
	defer conversationMutex.RUnlock()

	c := getConversation(conversationId)
	if c == nil {
		return true
	}
	if c.Position >= len(c.ActionList) {
		Destroy(conversationId)
		return true
	}
	return false
}

func GetNextActions(convId int) (mob1Id int, mob2Id int, actions []string) {
	conversationMutex.RLock()
	// RUnlock is deferred within getConversation if found, or here if not

	c := getConversation(convId)
	if c == nil {
		conversationMutex.RUnlock()
		return 0, 0, []string{}
	}
	// Defer RUnlock now that we know 'c' is valid
	defer conversationMutex.RUnlock()

	mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Getting next actions for conversation %d", convId))

	// First validation pass - we only care if it's valid, not the mob instances
	_, _, valid := c.validateMobInstances()
	if !valid {
		mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Initial validation failed for conversation %d", convId))
		return 0, 0, []string{}
	}

	// Get instance IDs safely with revalidation
	var id1, id2 int
	if !c.IsPlayer1 {
		// Revalidate mob1 right before use
		mob1Interface := mobinterfaces.GetInstance(c.MobInstanceId1)
		if mob1Interface == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob1 became invalid between validation and use in conversation %d (ID: %d)", convId, c.MobInstanceId1))
			go func() { Destroy(convId) }()
			return 0, 0, []string{}
		}
		mob1Concrete, ok := mob1Interface.(*mobs.Mob)
		if !ok || mob1Concrete == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob1 type changed between validation and use in conversation %d (ID: %d)", convId, c.MobInstanceId1))
			go func() { Destroy(convId) }()
			return 0, 0, []string{}
		}
		id1 = mob1Concrete.GetInstanceId()
		if id1 == 0 {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob1 instance ID became 0 between validation and use in conversation %d", convId))
			go func() { Destroy(convId) }()
			return 0, 0, []string{}
		}
		mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Mob1 revalidated successfully: %s (ID: %d)", mob1Concrete.GetName(), id1))
	} else {
		id1 = c.MobInstanceId1
		mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Using player1 ID: %d", id1))
	}

	if !c.IsPlayer2 {
		// Revalidate mob2 right before use
		mob2Interface := mobinterfaces.GetInstance(c.MobInstanceId2)
		if mob2Interface == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob2 became invalid between validation and use in conversation %d (ID: %d)", convId, c.MobInstanceId2))
			go func() { Destroy(convId) }()
			return 0, 0, []string{}
		}
		mob2Concrete, ok := mob2Interface.(*mobs.Mob)
		if !ok || mob2Concrete == nil {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob2 type changed between validation and use in conversation %d (ID: %d)", convId, c.MobInstanceId2))
			go func() { Destroy(convId) }()
			return 0, 0, []string{}
		}
		id2 = mob2Concrete.GetInstanceId()
		if id2 == 0 {
			mudlog.Error("Conversation", "error", fmt.Sprintf("Mob2 instance ID became 0 between validation and use in conversation %d", convId))
			go func() { Destroy(convId) }()
			return 0, 0, []string{}
		}
		mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Mob2 revalidated successfully: %s (ID: %d)", mob2Concrete.GetName(), id2))
	} else {
		id2 = c.MobInstanceId2
		mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Using player2 ID: %d", id2))
	}

	// Get next actions
	na := c.NextActions(util.GetRoundCount())
	if len(na) == 0 {
		// If no actions, check if conversation is complete
		if c.Position >= len(c.ActionList) {
			mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Conversation %d complete, destroying", convId))
			go func() { Destroy(convId) }()
		}
		return id1, id2, []string{}
	}

	mudlog.Debug("Conversation", "GetNextActions", fmt.Sprintf("Returning %d actions for conversation %d", len(na), convId))
	return id1, id2, na
}

type Conversation struct {
	Id             int
	MobInstanceId1 int    // For mob1 (if it's a mob)
	MobInstanceId2 int    // For mob2 (if it's a mob)
	PlayerName1    string // For participant1 (if it's a player)
	PlayerName2    string // For participant2 (if it's a player)
	IsPlayer1      bool   // Whether participant1 is a player
	IsPlayer2      bool   // Whether participant2 is a player
	StartRound     uint64
	LastRound      uint64
	// What the actions are and where we are in them
	Position   int
	ActionList [][]string
	// LLM-specific fields
	LLMConfig   *LLMConversationConfig
	Context     []string // Conversation history for LLM context
	LastLLMTime time.Time
	LLMCooldown time.Duration // Minimum time between LLM calls
}

func ZoneNameSanitize(zone string) string {
	if zone == "" {
		return ""
	}
	// Convert spaces to underscores
	zone = strings.ReplaceAll(zone, " ", "_")
	// Lowercase it all, and add a slash at the end
	return strings.ToLower(zone)
}

func HasConverseFile(mobId int, zone string) bool {

	zone = ZoneNameSanitize(zone)

	cacheKey := strconv.Itoa(mobId) + `-` + zone
	mudlog.Debug("HasConverseFile", "info", fmt.Sprintf("cachekey: %v", cacheKey))
	if result, ok := converseCheckCache[cacheKey]; ok {
		if result == false {
			return false
		}
	}

	convFolder := string(configs.GetFilePathsConfig().DataFiles) + `/conversations`

	fileName := fmt.Sprintf("%s/%d.yaml", zone, mobId)

	filePath := util.FilePath(convFolder + `/` + fileName)

	if _, err := os.Stat(filePath); err != nil {
		converseCheckCache[cacheKey] = false
		return false
	}

	converseCheckCache[cacheKey] = true

	return true

}

// validateMobInstances checks if the mob participants in a conversation are still valid
// Returns the mob instances if valid, and player names for player participants
func (c *Conversation) validateMobInstances() (mob1 *mobs.Mob, mob2 *mobs.Mob, valid bool) {
	select {
	case <-shutdownChan:
		return nil, nil, false
	default:
		mudlog.Debug("Conversation", "validate", fmt.Sprintf("Validating conversation %d: Mob1(ID:%d, IsPlayer:%v), Mob2(ID:%d, IsPlayer:%v)",
			c.Id, c.MobInstanceId1, c.IsPlayer1, c.MobInstanceId2, c.IsPlayer2))

		// Handle mob1
		var mob1Interface mobinterfaces.MobInterface
		if !c.IsPlayer1 {
			mob1Interface = mobinterfaces.GetInstance(c.MobInstanceId1)
			mudlog.Debug("Conversation", "validate", fmt.Sprintf("Mob1 instance lookup: ID=%d, Found=%v", c.MobInstanceId1, mob1Interface != nil))

			if mob1Interface == nil {
				mudlog.Error("Conversation", "error", fmt.Sprintf("Mob1 instance not found in conversation %d (ID: %d)", c.Id, c.MobInstanceId1))
				go func() { Destroy(c.Id) }()
				return nil, nil, false
			}

			mob1Concrete, ok := mob1Interface.(*mobs.Mob)
			if !ok || mob1Concrete == nil {
				mudlog.Error("Conversation", "error", fmt.Sprintf("Invalid mob1 type in conversation %d (ID: %d)", c.Id, c.MobInstanceId1))
				go func() { Destroy(c.Id) }()
				return nil, nil, false
			}
			mob1 = mob1Concrete
			mudlog.Debug("Conversation", "validate", fmt.Sprintf("Mob1 validated: %s", mob1.GetName()))
		}

		// Handle mob2
		var mob2Interface mobinterfaces.MobInterface
		if !c.IsPlayer2 {
			mob2Interface = mobinterfaces.GetInstance(c.MobInstanceId2)
			mudlog.Debug("Conversation", "validate", fmt.Sprintf("Mob2 instance lookup: ID=%d, Found=%v", c.MobInstanceId2, mob2Interface != nil))

			if mob2Interface == nil {
				mudlog.Error("Conversation", "error", fmt.Sprintf("Mob2 instance not found in conversation %d (ID: %d)", c.Id, c.MobInstanceId2))
				go func() { Destroy(c.Id) }()
				return nil, nil, false
			}

			mob2Concrete, ok := mob2Interface.(*mobs.Mob)
			if !ok || mob2Concrete == nil {
				mudlog.Error("Conversation", "error", fmt.Sprintf("Invalid mob2 type in conversation %d (ID: %d)", c.Id, c.MobInstanceId2))
				go func() { Destroy(c.Id) }()
				return nil, nil, false
			}
			mob2 = mob2Concrete
			mudlog.Debug("Conversation", "validate", fmt.Sprintf("Mob2 validated: %s", mob2.GetName()))
		}

		// Verify names for mobs
		if mob1 != nil {
			name := mob1.GetName()
			if name == "" {
				mudlog.Error("Conversation", "error", fmt.Sprintf("Invalid mob1 name in conversation %d (ID: %d)", c.Id, c.MobInstanceId1))
				go func() { Destroy(c.Id) }()
				return nil, nil, false
			}
			mudlog.Debug("Conversation", "validate", fmt.Sprintf("Mob1 name verified: %s", name))
		}

		if mob2 != nil {
			name := mob2.GetName()
			if name == "" {
				mudlog.Error("Conversation", "error", fmt.Sprintf("Invalid mob2 name in conversation %d (ID: %d)", c.Id, c.MobInstanceId2))
				go func() { Destroy(c.Id) }()
				return nil, nil, false
			}
			mudlog.Debug("Conversation", "validate", fmt.Sprintf("Mob2 name verified: %s", name))
		}

		mudlog.Debug("Conversation", "validate", fmt.Sprintf("Conversation %d validation successful", c.Id))
		return mob1, mob2, true
	}
}

func (c *Conversation) NextActions(roundNow uint64) []string {
	if c.LastRound == roundNow {
		return []string{}
	}

	c.LastRound = roundNow

	// Validate mob instances before proceeding
	mob1, mob2, valid := c.validateMobInstances()
	if !valid {
		return []string{}
	}

	pos := c.Position
	if c.Position >= len(c.ActionList) {
		return []string{}
	}

	c.Position++

	actions := append([]string{}, c.ActionList[pos]...)

	// If LLM is enabled for this conversation, process any LLM actions
	if c.LLMConfig != nil && c.LLMConfig.Enabled && bool(configs.GetIntegrationsConfig().LLM.Enabled) {
		// Get names for both participants
		name1 := c.PlayerName1
		if !c.IsPlayer1 && mob1 != nil {
			name1 = mob1.GetName()
		}
		name2 := c.PlayerName2
		if !c.IsPlayer2 && mob2 != nil {
			name2 = mob2.GetName()
		}
		actions = c.processLLMActions(actions, name1, name2)
	}

	return actions
}

// processLLMActions handles LLM-specific actions in the conversation
// It now receives concrete *mobs.Mob types
func (c *Conversation) processLLMActions(actions []string, playerName1 string, playerName2 string) []string {
	select {
	case <-shutdownChan:
		return actions
	default:
		// The calling function (NextActions) already validated mob1 and mob2 are non-nil
		// But we re-validate inside the loop before LLM calls for extra safety

		// Check cooldown
		if time.Since(c.LastLLMTime) < c.LLMCooldown {
			mudlog.Debug("LLM", "info", fmt.Sprintf("Skipping LLM call due to cooldown. Time since last call: %v", time.Since(c.LastLLMTime)))
			return actions
		}

		// Process each action
		for i, action := range actions {
			if len(action) < 3 {
				continue
			}

			target := action[0:3]
			cmd := action[3:]

			// Check if this is an LLM action
			if strings.HasPrefix(cmd, "llm ") {
				mudlog.Info("LLM", "info", fmt.Sprintf("Processing LLM action: %s", action))

				// Extract the prompt
				prompt := strings.TrimPrefix(cmd, "llm ")

				// Build context using the validated mob instances
				context, err := c.buildLLMContext(playerName1, playerName2) // Use newly validated mobs
				if err != nil {
					mudlog.Error("LLM", "error", fmt.Sprintf("Failed to build context: %v", err))
					actions[i] = "" // Clear action on error
					continue
				}
				mudlog.Debug("LLM", "context", fmt.Sprintf("Context for LLM: %v", context))

				// Generate response
				response := llm.GenerateResponse(prompt, context)
				if response.Error != nil {
					mudlog.Error("LLM", "error", fmt.Sprintf("Failed to generate response: %v", response.Error))
					actions[i] = "" // Clear action on error
					continue
				}

				mudlog.Info("LLM", "response", fmt.Sprintf("Generated response: %s", response.Text))

				// Update the action with the generated response
				if target == "#1 " {
					actions[i] = fmt.Sprintf("#1 sayto #2 %s", response.Text)
				} else {
					actions[i] = fmt.Sprintf("#2 sayto #1 %s", response.Text)
				}

				// Update context and cooldown
				c.Context = append(c.Context, response.Text)
				if len(c.Context) > c.LLMConfig.MaxContextTurns {
					c.Context = c.Context[len(c.Context)-c.LLMConfig.MaxContextTurns:]
				}
				c.LastLLMTime = time.Now()
			}
		}

		// Filter out any cleared actions (optional, depends on desired behavior)
		validActions := make([]string, 0, len(actions))
		for _, act := range actions {
			if act != "" {
				validActions = append(validActions, act)
			}
		}

		return validActions
	}
}

// buildLLMContext creates the context for the LLM based on conversation history
// It now takes mob names as strings instead of mob objects
func (c *Conversation) buildLLMContext(mob1Name string, mob2Name string) ([]string, error) {
	// Validate mob names
	if mob1Name == "" || mob2Name == "" {
		return nil, fmt.Errorf("invalid mob names in buildLLMContext: mob1=%q, mob2=%q", mob1Name, mob2Name)
	}

	context := make([]string, 0)

	// Add system prompt if provided
	if c.LLMConfig != nil && c.LLMConfig.SystemPrompt != "" {
		context = append(context, c.LLMConfig.SystemPrompt)
		mudlog.Debug("LLM", "context", fmt.Sprintf("Added system prompt: %s", c.LLMConfig.SystemPrompt))
	}

	// Add NPC names if enabled
	if c.LLMConfig != nil && c.LLMConfig.IncludeNames {
		context = append(context, fmt.Sprintf("NPC1: %s", mob1Name))
		context = append(context, fmt.Sprintf("NPC2: %s", mob2Name))
		mudlog.Debug("LLM", "context", fmt.Sprintf("Added NPC names: %s, %s", mob1Name, mob2Name))
	}

	// Add conversation history
	context = append(context, c.Context...)
	if len(c.Context) > 0 {
		mudlog.Debug("LLM", "context", fmt.Sprintf("Added conversation history: %v", c.Context))
	}

	return context, nil
}

func getConversation(conversationId int) *Conversation {
	// Note: This function assumes the caller holds the appropriate lock
	select {
	case <-shutdownChan:
		return nil
	default:
		// Only do maintenance if not shutting down
		if util.Rand(50) == 0 { // 2% chance to do a quick maintenance
			rNow := util.GetRoundCount()
			for id, info := range conversations {
				// Check if mobs are still valid
				mob1 := mobinterfaces.GetInstance(info.MobInstanceId1)
				mob2 := mobinterfaces.GetInstance(info.MobInstanceId2)
				if mob1 == nil || mob2 == nil || rNow-info.LastRound > 10 {
					// During shutdown, destroy directly
					select {
					case <-shutdownChan:
						destroyConversation(id)
					default:
						go func(id int) {
							select {
							case <-shutdownChan:
								destroyConversation(id)
							default:
								Destroy(id)
							}
						}(id)
					}
				}
			}
		}

		if conversation, ok := conversations[conversationId]; ok {
			return conversation
		}

		return nil
	}
}

// GetConversation returns the conversation by ID, or nil if not found.
func GetConversation(conversationId int) *Conversation {
	conversationMutex.RLock()
	defer conversationMutex.RUnlock()
	return getConversation(conversationId)
}
