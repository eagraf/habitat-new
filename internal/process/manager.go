package process

import (
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
)

type RunningProcess struct {
	*node.ProcessState
}

type ProcessManager interface {
	ListProcesses() ([]*node.ProcessState, error)
	StartProcess(*node.Process, *node.AppInstallation) error
	StopProcess(extProcessID string) error
	GetProcess(processID string) (*node.ProcessState, error)
}

type baseProcessManager struct {
	processDrivers map[string]Driver

	processes map[string]*RunningProcess
}

func NewProcessManager(drivers []Driver) ProcessManager {
	pm := &baseProcessManager{
		processDrivers: make(map[string]Driver),
		processes:      make(map[string]*RunningProcess),
	}
	for _, driver := range drivers {
		pm.processDrivers[driver.Type()] = driver
	}
	return pm
}

func (pm *baseProcessManager) ListProcesses() ([]*node.ProcessState, error) {
	processList := make([]*node.ProcessState, 0, len(pm.processes))
	for _, process := range pm.processes {
		processList = append(processList, process.ProcessState)
	}

	return processList, nil
}

func (pm *baseProcessManager) GetProcess(processID string) (*node.ProcessState, error) {
	proc, ok := pm.processes[processID]
	if !ok {
		return nil, fmt.Errorf("error getting process: process %s not found", processID)
	}
	return proc.ProcessState, nil
}

func (pm *baseProcessManager) StartProcess(process *node.Process, app *node.AppInstallation) error {
	proc, ok := pm.processes[process.ID]
	if ok {
		return fmt.Errorf("error starting process: process %s already found: %v", process.ID, proc)
	}

	driver, ok := pm.processDrivers[app.Driver]
	if !ok {
		return fmt.Errorf("error starting process: driver %s not found", app.Driver)
	}

	extProcessID, err := driver.StartProcess(process, app)
	if err != nil {
		return err
	}

	pm.processes[process.ID] = &RunningProcess{
		ProcessState: &node.ProcessState{
			Process:     process,
			ExtDriverID: extProcessID,
		},
	}

	// TODO tell controller that we're in state running
	return nil
}

func (pm *baseProcessManager) StopProcess(processID string) error {
	process, ok := pm.processes[processID]
	if !ok {
		return fmt.Errorf("error stopping process: process %s not found", processID)
	}

	driver, ok := pm.processDrivers[process.Driver]
	if !ok {
		return fmt.Errorf("error stopping process: driver %s not found", process.Driver)
	}

	err := driver.StopProcess(process.ExtDriverID)
	if err != nil {
		return err
	}

	delete(pm.processes, processID)

	return nil
}
