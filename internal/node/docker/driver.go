package docker

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
	"github.com/rs/zerolog/log"
)

type DockerDriver struct {
	client *client.Client
}

// Implement the package manager interface
func (d *DockerDriver) InstallPackage(packageSpec *package_manager.PackageSpec, version string) error {
	if packageSpec.DriverType != "docker" {
		return fmt.Errorf("invalid package driver: %s, expected docker", packageSpec.DriverType)
	}

	registryURL := fmt.Sprintf("%s/%s:%s", packageSpec.RegistryURLBase, packageSpec.RegistryPackageID, packageSpec.RegistryPackageTag)
	_, err := d.client.ImagePull(context.Background(), registryURL, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	log.Info().Msgf("Pulled image %s", registryURL)
	return nil
}

func (d *DockerDriver) UninstallPackage(packageURL *package_manager.PackageSpec, version string) error {
	return errors.New("not implemented")
}
