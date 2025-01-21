package docker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/rs/zerolog/log"
)

const (
	habitatLabel   = "habitat_proc_id"
	errNoProcFound = "no process found with id %s"
)

type dockerDriver struct {
	client *client.Client
}

// dockerDriver implements process.Driver
var _ process.Driver = &dockerDriver{}

func NewDriver(client *client.Client) process.Driver {
	return &dockerDriver{
		client: client,
	}
}

func (d *dockerDriver) Type() string {
	return constants.AppDriverDocker
}

// StartProcess helps implement dockerDriver
func (d *dockerDriver) StartProcess(ctx context.Context, process *node.Process, app *node.AppInstallation) error {
	var dockerConfig node.AppInstallationConfig
	dockerConfigBytes, err := json.Marshal(app.DriverConfig)
	if err != nil {
		return err
	}

	err = json.Unmarshal(dockerConfigBytes, &dockerConfig)
	if err != nil {
		return err
	}

	exposedPorts := make(nat.PortSet)
	for _, port := range dockerConfig.ExposedPorts {
		exposedPorts[nat.Port(port)] = struct{}{}
	}

	createResp, err := d.client.ContainerCreate(ctx, &container.Config{
		Image:        fmt.Sprintf("%s/%s:%s", app.RegistryURLBase, app.RegistryPackageID, app.RegistryPackageTag),
		ExposedPorts: exposedPorts,
		Env:          dockerConfig.Env,
		Labels: map[string]string{
			habitatLabel: string(process.ID),
		},
	}, &container.HostConfig{
		PortBindings: dockerConfig.PortBindings,
		Mounts:       dockerConfig.Mounts,
	}, nil, nil, "")
	if err != nil {
		return err
	}

	err = d.client.ContainerStart(ctx, createResp.ID, container.StartOptions{})
	if err != nil {
		return err
	}

	log.Info().Msgf("Started docker container %s", createResp.ID)

	return nil
}

func (d *dockerDriver) StopProcess(ctx context.Context, processID node.ProcessID) error {
	ctrs, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	var ctr *types.Container
	for _, c := range ctrs {
		if id, ok := c.Labels[string(habitatLabel)]; ok && id == string(processID) {
			ctr = &c
			break
		}
	}
	if ctr == nil {
		return fmt.Errorf(errNoProcFound, processID)
	}

	err = d.client.ContainerStop(ctx, ctr.ID, container.StopOptions{})
	if err != nil {
		return err
	}

	return nil
}
