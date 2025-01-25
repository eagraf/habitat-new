package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/package_manager"
	"github.com/rs/zerolog/log"
)

// Example docker driver configuration for a container configuration:

/**
	"driver_config": {
		"env": [
			"PDS_HOSTNAME=ethangraf.com",
			"PDS_DATA_DIRECTORY=/pds",
		],
		"mounts": [
			{
				"Type": "bind",
				"Source": "/Users/ethan/code/habitat/habitat-new/.habitat/pds",
				"Target": "/pds"
			}
		],
		"exposed_ports": [ "5000" ],
		"port_bindings": {
			"3000/tcp": [
				{
					"hostIp": "127.0.0.1",
					"hostPort": "5000"
				}
			]
		}
	}
**/

type dockerPackageManager struct {
	client *client.Client
}

// dockerPackageManager implements PackageManager
var _ package_manager.PackageManager = &dockerPackageManager{}

func NewPackageManager(client *client.Client) package_manager.PackageManager {
	return &dockerPackageManager{
		client: client,
	}
}

func (d *dockerPackageManager) Driver() node.Driver {
	return node.DriverDocker
}

func repoURLFromPackage(packageSpec *node.Package) string {
	return fmt.Sprintf("%s/%s:%s", packageSpec.RegistryURLBase, packageSpec.RegistryPackageID, packageSpec.RegistryPackageTag)
}

func (d *dockerPackageManager) IsInstalled(packageSpec *node.Package, version string) (bool, error) {
	// TODO review all contexts we create.
	repoURL := repoURLFromPackage(packageSpec)
	images, err := d.client.ImageList(context.Background(), types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("reference", repoURL),
		),
	})
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

// Implement the package manager interface
func (d *dockerPackageManager) InstallPackage(packageSpec *node.Package, version string) error {
	if packageSpec.Driver != node.DriverDocker {
		return fmt.Errorf("invalid package driver: %s, expected docker", packageSpec.Driver)
	}

	repoURL := repoURLFromPackage(packageSpec)
	_, err := d.client.ImagePull(context.Background(), repoURL, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	log.Info().Msgf("Pulled image %s", repoURL)
	return nil
}

func (d *dockerPackageManager) UninstallPackage(packageURL *node.Package, version string) error {
	repoURL := repoURLFromPackage(packageURL)
	_, err := d.client.ImageRemove(context.Background(), repoURL, types.ImageRemoveOptions{})
	if err != nil {
		return err
	}
	return nil
}
