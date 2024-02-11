package node

// Core structs for the node state. These are intended to be embedable in other structs
// throughout the application. That way, it's easy to modify the core struct, while having
// the component specific structs to be decoupled. Fields in these structs should be immutable.

// TODO to make these truly immutable, only methods should be exported, all fields should be private.

const AppLifecycleStateInstalling = "installing"
const AppLifecycleStateInstalled = "installed"

// TODO some fields should be ignored by the REST api
type AppInstallation struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Driver          string `json:"driver"`
	RegistryURLBase string `json:"registry_url_base"`
	RegistryAppID   string `json:"registry_app_id"`
	RegistryTag     string `json:"registry_tag"`
}

const ProcessStateStarting = "starting"
const ProcessStateRunning = "running"

type Process struct {
	ID      string `json:"id"`
	AppID   string `json:"app_id"`
	UserID  string `json:"user_id"`
	Created string `json:"created"`
}
