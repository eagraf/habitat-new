package controller

import (
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
)

func (c *NodeController) InstallApp(userID, appName, version, driver string, packageSpec *package_manager.PackageSpec) error {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = db.Controller.ProposeTransitions([]state.Transition{
		&node.StartInstallationTransition{
			UserID:          userID,
			Name:            appName,
			Version:         version,
			Driver:          driver,
			RegistryURLBase: packageSpec.RegistryURLBase,
			RegistryAppID:   packageSpec.RegistryPackageID,
			RegistryTag:     packageSpec.RegistryPackageTag,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
