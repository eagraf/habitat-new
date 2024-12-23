package node

import (
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
)

// TODO some fields should be ignored by the REST api
type AppInstallation struct {
	ID      string `json:"id" yaml:"id"`
	UserID  string `json:"user_id" yaml:"user_id"`
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Package `yaml:",inline"`
}

// AppInstallationConfig is a struct to hold the configuration for a docker container
// Most of these types are taken directly from the Docker Go SDK
type AppInstallationConfig struct {
	// ExposedPorts is a slice of ports exposed by the docker container
	ExposedPorts []string `json:"exposed_ports"`
	// Env is a slice of environment variables to be set in the container, specified as KEY=VALUE
	Env []string `json:"env"`
	// PortBindings is a map of ports to bind on the host to ports in the container. Host IPs can be specified as well
	PortBindings nat.PortMap `json:"port_bindings"`
	// Mounts is a slice of mounts to be mounted in the container
	Mounts []mount.Mount `json:"mounts"`
}
