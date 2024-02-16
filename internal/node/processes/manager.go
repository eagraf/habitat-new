package processes

import (
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
)

type RunningProcess struct {
	node.Process
}

type ProcessManager interface {
	ListProcesses() ([]*node.Process, error)
	StartProcess(*node.Process, *node.AppInstallation) error
	StopProcess(extProcessID string) error
	GetProcess(processID string) (*node.Process, error)
}

type ProcessDriver interface {
	Type() string
	StartProcess(*node.Process, *node.AppInstallation) (string, error)
	StopProcess(extProcessID string) error
}

type BaseProcessManager struct {
	processDrivers map[string]ProcessDriver

	processes map[string]*RunningProcess
}

func newBaseProcessManager() *BaseProcessManager {
	return &BaseProcessManager{
		processDrivers: make(map[string]ProcessDriver),
		processes:      make(map[string]*RunningProcess),
	}
}

func (pm *BaseProcessManager) ListProcesses() ([]*node.Process, error) {
	processList := make([]*node.Process, 0, len(pm.processes))
	for _, process := range pm.processes {
		processList = append(processList, &process.Process)
	}

	return processList, nil
}

func (pm *BaseProcessManager) GetProcess(processID string) (*node.Process, error) {
	proc, ok := pm.processes[processID]
	if !ok {
		return nil, fmt.Errorf("error getting process: process %s not found", processID)
	}
	return &proc.Process, nil
}

func (pm *BaseProcessManager) StartProcess(process *node.Process, app *node.AppInstallation) error {
	driver, ok := pm.processDrivers[app.Driver]
	if !ok {
		return fmt.Errorf("error starting process: driver %s not found", app.Driver)
	}

	_, err := driver.StartProcess(process, app)
	if err != nil {
		return err
	}

	pm.processes[process.ID] = &RunningProcess{
		Process: *process,
	}

	// TODO tell controller that we're in state running

	return nil
}

func (pm *BaseProcessManager) StopProcess(processID string) error {
	process, ok := pm.processes[processID]
	if !ok {
		return fmt.Errorf("error stopping process: process %s not found", processID)
	}

	driver, ok := pm.processDrivers[process.Driver]
	if !ok {
		return fmt.Errorf("error stopping process: driver %s not found", process.Driver)
	}

	err := driver.StopProcess(processID)
	if err != nil {
		return err
	}

	delete(pm.processes, processID)

	return nil
}
