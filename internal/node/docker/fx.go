package docker

import (
	"github.com/rs/zerolog/log"

	"github.com/docker/docker/client"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
	"github.com/eagraf/habitat-new/internal/node/processes"
	"go.uber.org/fx"
)

type DriverResult struct {
	fx.Out
	PackageManager package_manager.PackageManager
	ProcessDriver  processes.ProcessDriver `group:"process_drivers"`
}

func NewDockerDriver() (DriverResult, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create docker client")
	}

	res := DriverResult{
		PackageManager: &AppDriver{
			client: dockerClient,
		},
		ProcessDriver: &ProcessDriver{
			client: dockerClient,
		},
	}

	return res, nil
}
