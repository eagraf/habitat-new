package package_manager

import (
	"encoding/json"
	"errors"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
)

type AppLifecycleSubscriber struct {
	packageManager PackageManager
}

func (s *AppLifecycleSubscriber) ConsumeEvent(event *state.StateUpdate) error {
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
}
