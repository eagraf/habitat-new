package package_manager

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

/*type AppLifecycleSubscriber struct {
	packageManager PackageManager
	nodeController controller.NodeController
}

func (s *AppLifecycleSubscriber) Name() string {
	return "AppLifecycleSubscriber"
}

func (s *AppLifecycleSubscriber) ConsumeEvent(event *hdb.StateUpdate) error {
	// TODO eventually the pubsub system will support subscribing to topics
	if event.SchemaType != node.SchemaName {
		return nil
	}

	switch event.TransitionType {
	case node.TransitionStartInstallation:
		var t node.StartInstallationTransition
		err := json.Unmarshal(event.Transition, &t)
		if err != nil {
			return err
		}
		err = s.packageManager.InstallPackage(&PackageSpec{
			DriverType:         t.Driver,
			RegistryURLBase:    t.RegistryURLBase,
			RegistryPackageID:  t.RegistryAppID,
			RegistryPackageTag: t.RegistryTag,
		}, t.Version)
		if err != nil {
			return err
		}

		// After finishing the installation, update the application's lifecycle state
		err = s.nodeController.FinishAppInstallation(t.UserID, t.RegistryURLBase, t.RegistryAppID)
		if err != nil {
			return err
		}

		return err
	case node.TransitionFinishInstallation:
		// noop
		return nil
	case node.TransitionStartUninstallation:
		// TODO uninstalling not implemented yet
		return errors.New("uninstalling not implemented yet")
	default:
		return nil
	}
}*/

type InstallAppExecutor struct {
	packageManager PackageManager
	nodeController controller.NodeController
}

func (e *InstallAppExecutor) TransitionType() string {
	return node.TransitionStartInstallation
}

func (e *InstallAppExecutor) ShouldExecute(update *hdb.StateUpdate) (bool, error) {
	var t node.StartInstallationTransition
	err := json.Unmarshal(update.Transition, &t)
	if err != nil {
		return false, err
	}
	spec := &PackageSpec{
		DriverType:         t.Driver,
		RegistryURLBase:    t.RegistryURLBase,
		RegistryPackageID:  t.RegistryAppID,
		RegistryPackageTag: t.RegistryTag,
	}

	return e.packageManager.IsInstalled(spec, t.Version)
}

func (e *InstallAppExecutor) Execute(update *hdb.StateUpdate) error {
	var t node.StartInstallationTransition
	err := json.Unmarshal(update.Transition, &t)
	if err != nil {
		return err
	}
	err = e.packageManager.InstallPackage(&PackageSpec{
		DriverType:         t.Driver,
		RegistryURLBase:    t.RegistryURLBase,
		RegistryPackageID:  t.RegistryAppID,
		RegistryPackageTag: t.RegistryTag,
	}, t.Version)
	if err != nil {
		return err
	}

	// After finishing the installation, update the application's lifecycle state
	err = e.nodeController.FinishAppInstallation(t.UserID, t.RegistryURLBase, t.RegistryAppID)
	if err != nil {
		return err
	}

	return nil
}
