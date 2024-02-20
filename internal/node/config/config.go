package config

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	viper "github.com/spf13/viper"
)

func loadEnv() error {
	err := viper.BindEnv("habitat_path", "HABITAT_PATH")
	if err != nil {
		return err
	}
	viper.SetDefault("habitat_path", "$HOME/.habitat")

	err = viper.BindEnv("habitat_app_path")
	if err != nil {
		return err
	}
	return nil
}

func loadConfig() (*NodeConfig, error) {
	viper.AddConfigPath("$HOME/.habitat")
	viper.AddConfigPath(viper.GetString("habitat_path"))

	viper.SetConfigType("yml")
	viper.SetConfigName("habitat")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config NodeConfig
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Read cert files
	rootCert, err := decodePemCert(config.RootUserCertPath())
	if err != nil {
		return nil, err
	}
	config.rootUserCert = rootCert

	nodeCert, err := decodePemCert(config.NodeCertPath())
	if err != nil {
		return nil, err
	}
	config.nodeCert = nodeCert

	log.Debug().Msgf("Loaded node config: %+v", config)

	return &config, nil
}

func decodePemCert(certPath string) (*x509.Certificate, error) {
	pemBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("Got nil block after decoding PEM")
	}

	if block.Type != "CERTIFICATE" {
		return nil, errors.New("Expected CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

type NodeConfig struct {
	rootUserCert *x509.Certificate
	nodeCert     *x509.Certificate
}

func (n *NodeConfig) HabitatPath() string {
	return viper.GetString("habitat_path")
}

func (n *NodeConfig) HabitatAppPath() string {
	return viper.GetString("habitat_app_path")
}

func (n *NodeConfig) HDBPath() string {
	return filepath.Join(n.HabitatPath(), "hdb")
}

func (n *NodeConfig) NodeCertPath() string {
	return filepath.Join(n.HabitatPath(), "certificates", "dev_node_cert.pem")
}

func (n *NodeConfig) NodeKeyPath() string {
	return filepath.Join(n.HabitatPath(), "certificates", "dev_node_key.pem")
}

func (n *NodeConfig) RootUserCertPath() string {
	return filepath.Join(n.HabitatPath(), "certificates", "dev_root_user_cert.pem")
}

func (n *NodeConfig) RootUserCert() *x509.Certificate {
	return n.rootUserCert
}

func (n *NodeConfig) RootUserCertB64() string {
	return base64.StdEncoding.EncodeToString(n.rootUserCert.Raw)
}
