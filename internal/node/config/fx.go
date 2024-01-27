package config

func NewNodeConfig() (*NodeConfig, error) {
	loadEnv()
	return loadConfig()
}
