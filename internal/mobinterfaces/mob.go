package mobinterfaces

// MobInterface defines the minimal interface needed by the conversations package
type MobInterface interface {
	// GetInstanceId returns the instance ID of the mob
	GetInstanceId() int
	// GetName returns the name of the mob
	GetName() string
	// GetCharacter returns the character data of the mob
	GetCharacter() interface{} // Using interface{} to avoid circular dependencies
	// SetConversation sets the conversation ID for this mob
	SetConversation(id int)
	// InConversation returns whether this mob is currently in a conversation
	InConversation() bool
}

// GetInstance is a function type that will be implemented by the mobs package
type GetInstanceFunc func(instanceId int) MobInterface

var getInstance GetInstanceFunc

// RegisterGetInstance allows the mobs package to register its implementation
func RegisterGetInstance(fn GetInstanceFunc) {
	getInstance = fn
}

// GetInstance is the public function that conversations will use
func GetInstance(instanceId int) MobInterface {
	if getInstance == nil {
		return nil
	}
	return getInstance(instanceId)
}
