package node

// Core structs for the node state. These are intended to be embedable in other structs
// throughout the application. That way, it's easy to modify the core struct, while having
// the component specific structs to be decoupled.

const AppLifecycleStateInstalling = "installing"

type AppInstallation struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Driver          string `json:"driver"`
	RegistryURLBase string `json:"registry_url_base"`
	RegistryAppID   string `json:"registry_app_id"`
	RegistryTag     string `json:"registry_tag"`
}
