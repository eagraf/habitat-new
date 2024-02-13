package package_manager

import (
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

func NewAppLifecycleSubscriber(packageManager PackageManager, nodeController controller.NodeController) (*hdb.IdempotentStateUpdateSubscriber, error) {
	// TODO this should have a fx cleanup hook to cleanly handle interrupted installs
	// when the node shuts down.

	return hdb.NewIdempotentStateUpdateSubscriber(
		"AppLifecycleSubscriber",
		node.SchemaName,
		[]hdb.IdempotentStateUpdateExecutor{
			&InstallAppExecutor{
				packageManager: packageManager,
				nodeController: nodeController,
			},
		},
	)
}
