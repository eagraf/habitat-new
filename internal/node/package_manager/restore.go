package package_manager

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

type PackageManagerRestorer struct {
	packageManager PackageManager
}

func (r *PackageManagerRestorer) Restore(restoreEvent *hdb.StateUpdate) error {
	var nodeState node.NodeState
	err := json.Unmarshal(restoreEvent.NewState, &nodeState)
	if err != nil {
		return err
	}

	for _, app := range nodeState.AppInstallations {
		// Only try to install the app if it was in the state "installing"
		if app.State == node.AppLifecycleStateInstalling {
			err = r.packageManager.InstallPackage(&app.Package, app.Version)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
