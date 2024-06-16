package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"os/user"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	viper "github.com/spf13/viper"
)

func loadEnv() error {
	err := viper.BindEnv("debug", "DEBUG")
	if err != nil {
		return err
	}
	viper.SetDefault("debug", "DEBUG")

	err = viper.BindEnv("habitat_path", "HABITAT_PATH")
	if err != nil {
		return err
	}
	homedir, err := homedir()
	if err != nil {
		return err
	}
	viper.SetDefault("habitat_path", filepath.Join(homedir, ".habitat"))

	err = viper.BindEnv("habitat_app_path")
	if err != nil {
		return err
	}

	err = viper.BindEnv("use_tls", "USE_TLS")
	if err != nil {
		return err
	}
	viper.SetDefault("use_tls", false)

	err = viper.BindEnv("tailscale_authkey", "TS_AUTHKEY")
	if err != nil {
		return err
	}

	err = viper.BindEnv("tailnet", "TS_TAILNET")
	if err != nil {
		return err
	}

	err = viper.BindEnv("frontend_dev", "FRONTEND_DEV")
	if err != nil {
		return err
	}
	viper.SetDefault("frontend_dev", false)

	return nil
}

func loadConfig() (*NodeConfig, error) {
	homedir, err := homedir()
	if err != nil {
		return nil, err
	}

	viper.AddConfigPath(filepath.Join(homedir, ".habitat"))
	viper.AddConfigPath(viper.GetString("habitat_path"))

	viper.SetConfigType("yml")
	viper.SetConfigName("habitat")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config NodeConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Read cert files
	rootCert, err := decodePemCert(config.RootUserCertPath())
	if err != nil {
		return nil, err
	}
	config.RootUserCert = rootCert

	nodeCert, err := decodePemCert(config.NodeCertPath())
	if err != nil {
		return nil, err
	}
	config.NodeCert = nodeCert

	log.Debug().Msgf("Loaded node config: node cert: %s root cert: %s node key: %s", config.NodeCertPath(), config.RootUserCertPath(), config.NodeKeyPath())

	return &config, nil
}

func decodePemCert(certPath string) (*x509.Certificate, error) {
	pemBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("got nil block after decoding PEM")
	}

	if block.Type != "CERTIFICATE" {
		return nil, errors.New("expected CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

type NodeConfig struct {
	RootUserCert *x509.Certificate
	NodeCert     *x509.Certificate
}

func NewNodeConfig() (*NodeConfig, error) {
	err := loadEnv()
	if err != nil {
		return nil, err
	}
	return loadConfig()
}

func (n *NodeConfig) LogLevel() zerolog.Level {
	isDebug := viper.GetBool("debug")
	if isDebug {
		return zerolog.DebugLevel
	}
	return zerolog.InfoLevel
}

func (n *NodeConfig) HabitatPath() string {
	return viper.GetString("habitat_path")
}

func (n *NodeConfig) HabitatAppPath() string {
	// Note that in dev mode, this should point to a path on the host machine rather than in the Docker container.
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

func (n *NodeConfig) RootUserCertB64() string {
	return base64.StdEncoding.EncodeToString(n.RootUserCert.Raw)
}

func (n *NodeConfig) TLSConfig() (*tls.Config, error) {
	if !n.UseTLS() {
		return nil, nil
	}

	rootCertBytes, err := os.ReadFile(n.RootUserCertPath())
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCertBytes)

	return &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}

func (n *NodeConfig) UseTLS() bool {
	return viper.GetBool("use_tls")
}

// Currently unused, but may be necessary to implement adding members to the community.
func (n *NodeConfig) TailnetName() string {
	return viper.GetString("tailnet")
}

func (n *NodeConfig) TailscaleAuthkey() string {
	return viper.GetString("tailscale_authkey")
}

func (n *NodeConfig) TailScaleStatePath() string {
	// Note: this is intentionally not configurable for simplicity's sake.
	return filepath.Join(n.HabitatPath(), "tailscale_state")
}

func (n *NodeConfig) FrontendDev() bool {
	return viper.GetBool("frontend_dev")
}

func homedir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := usr.HomeDir
	return dir, nil
}
