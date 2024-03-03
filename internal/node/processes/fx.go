package processes

import (
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

func NewProcessManager(drivers []ProcessDriver) ProcessManager {
	pm := newBaseProcessManager()
	for _, driver := range drivers {
		pm.processDrivers[driver.Type()] = driver
	}
	return pm
}

func NewProcessManagerStateUpdateSubscriber(processManager ProcessManager, controller controller.NodeController) (*hdb.IdempotentStateUpdateSubscriber, error) {
	return hdb.NewIdempotentStateUpdateSubscriber(
		"StartProcessSubscriber",
		node.SchemaName,
		[]hdb.IdempotentStateUpdateExecutor{
			&StartProcessExecutor{
				processManager: processManager,
			},
		},
		&ProcessRestorer{
			processManager: processManager,
			nodeController: controller,
		},
	)
}
