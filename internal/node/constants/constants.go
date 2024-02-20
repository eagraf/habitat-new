package constants

type HabitatContextKey string

const (
	RootUsername      = "root"
	RootUserID        = "0"
	NodeDBDefaultName = "node"

	// Request context keys
	ContextKeyUserID HabitatContextKey = "user_id"

	// App driver names
	AppDriverDocker = "docker"
)
