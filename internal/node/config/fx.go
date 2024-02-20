package config

func NewNodeConfig() (*NodeConfig, error) {
	err := loadEnv()
	if err != nil {
		return nil, err
	}
	return loadConfig()
}
