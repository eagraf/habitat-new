package processes

import (
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

type RunningProcess struct {
	*node.ProcessState
}

type ProcessManager interface {
	StartProcess(*node.Process, *node.AppInstallation) (string, error)
	StopProcess(extProcessID string) error
	GetProcess(processID string) (*node.ProcessState, error)
	IsProcessRunning(processID string) (bool, error)
}

type ProcessDriver interface {
	Type() string
	StartProcess(*node.Process, *node.AppInstallation) (string, error)
	StopProcess(extProcessID string) error
	IsProcessRunning(extProcessID string) (bool, error)
}

type BaseProcessManager struct {
	processDrivers map[string]ProcessDriver
	nodeController controller.NodeController
}

func NewProcessManager(drivers []ProcessDriver, nodeController controller.NodeController) ProcessManager {
	pm := &BaseProcessManager{
		processDrivers: make(map[string]ProcessDriver),
		nodeController: nodeController,
	}
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
				nodeController: controller,
			},
		},
		&ProcessRestorer{
			processManager: processManager,
			nodeController: controller,
		},
	)
}

func (pm *BaseProcessManager) GetProcess(processID string) (*node.ProcessState, error) {
	nodeState, err := pm.nodeController.GetNodeState()
	if err != nil {
		return nil, err
	}

	return nodeState.GetProcessByID(processID)
}

func (pm *BaseProcessManager) StartProcess(process *node.Process, app *node.AppInstallation) (string, error) {
	driver, ok := pm.processDrivers[app.Driver]
	if !ok {
		return "", fmt.Errorf("error starting process: driver %s not found", app.Driver)
	}

	extProcessID, err := driver.StartProcess(process, app)
	if err != nil {
		return "", err
	}

	return extProcessID, nil
}

func (pm *BaseProcessManager) StopProcess(processID string) error {
	nodeState, err := pm.nodeController.GetNodeState()
	if err != nil {
		return err
	}

	process, err := nodeState.GetProcessByID(processID)
	if err != nil {
		return fmt.Errorf("error stopping process: process %s not found", processID)
	}

	driver, ok := pm.processDrivers[process.Driver]
	if !ok {
		return fmt.Errorf("error stopping process: driver %s not found", process.Driver)
	}

	err = driver.StopProcess(process.ExtDriverID)
	if err != nil {
		return err
	}

	return nil
}

func (pm *BaseProcessManager) IsProcessRunning(processID string) (bool, error) {
	nodeState, err := pm.nodeController.GetNodeState()
	if err != nil {
		return false, err
	}

	process, err := nodeState.GetProcessByID(processID)
	if err != nil {
		return false, fmt.Errorf("error stopping process: process %s not found", processID)
	}

	driver, ok := pm.processDrivers[process.Driver]
	if !ok {
		return false, fmt.Errorf("error checking if process is running: driver %s not found", process.Driver)
	}

	return driver.IsProcessRunning(process.ExtDriverID)
}
