package config

import "tailscale.com/tsnet"

func NewNodeConfig() (*NodeConfig, error) {
	err := loadEnv()
	if err != nil {
		return nil, err
	}
	return loadConfig()
}

func NewTsnetServer() *tsnet.Server {
	return new(tsnet.Server)
}
