package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/rs/zerolog/log"
)

type DockerAppInstallationConfig struct {
	ExposedPorts []string      `json:"exposed_ports"`
	Env          []string      `json:"env"`
	PortBindings nat.PortMap   `json:"port_bindings"`
	Mounts       []mount.Mount `json:"mounts"`
}

type AppDriver struct {
	client *client.Client
}

func (d *AppDriver) IsInstalled(packageSpec *node.Package, version string) (bool, error) {
	// TODO review all contexts we create.
	images, err := d.client.ImageList(context.Background(), types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("reference", fmt.Sprintf("%s/%s:%s", packageSpec.RegistryURLBase, packageSpec.RegistryPackageID, packageSpec.RegistryPackageTag)),
		),
	})
	if err != nil {
		return false, err
	}
	return !(len(images) > 0), nil
}

// Implement the package manager interface
func (d *AppDriver) InstallPackage(packageSpec *node.Package, version string) error {
	if packageSpec.Driver != "docker" {
		return fmt.Errorf("invalid package driver: %s, expected docker", packageSpec.Driver)
	}

	registryURL := fmt.Sprintf("%s/%s:%s", packageSpec.RegistryURLBase, packageSpec.RegistryPackageID, packageSpec.RegistryPackageTag)
	_, err := d.client.ImagePull(context.Background(), registryURL, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	log.Info().Msgf("Pulled image %s", registryURL)
	return nil
}

func (d *AppDriver) UninstallPackage(packageURL *node.Package, version string) error {
	return errors.New("not implemented")
}

type ProcessDriver struct {
	client *client.Client
}

func (d *ProcessDriver) Type() string {
	return constants.AppDriverDocker
}

// StartProcess helps implement processes.ProcessDriver
func (d *ProcessDriver) StartProcess(process *node.Process, app *node.AppInstallation) (string, error) {

	var dockerConfig DockerAppInstallationConfig
	dockerConfigBytes, err := json.Marshal(app.DriverConfig)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(dockerConfigBytes, &dockerConfig)
	if err != nil {
		return "", err
	}

	exposedPorts := make(nat.PortSet)
	for _, port := range dockerConfig.ExposedPorts {
		exposedPorts[nat.Port(port)] = struct{}{}
	}

	createResp, err := d.client.ContainerCreate(context.Background(), &container.Config{
		Image:        fmt.Sprintf("%s/%s:%s", app.RegistryURLBase, app.RegistryPackageID, app.RegistryPackageTag),
		ExposedPorts: exposedPorts,
		Env:          dockerConfig.Env,
	}, &container.HostConfig{
		PortBindings: dockerConfig.PortBindings,
		Mounts:       dockerConfig.Mounts,
	}, nil, nil, "")
	if err != nil {
		return "", err
	}

	err = d.client.ContainerStart(context.Background(), createResp.ID, container.StartOptions{})
	if err != nil {
		return "", err
	}

	log.Info().Msgf("Started container %s", createResp.ID)

	return createResp.ID, nil
}

func (d *ProcessDriver) StopProcess(extProcessID string) error {
	err := d.client.ContainerStop(context.Background(), extProcessID, container.StopOptions{})
	if err != nil {
		return err
	}

	return nil
}
