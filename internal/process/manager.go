package process

import (
	"context"
	"errors"
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

// Given RestoreInfo, the ProcessManager will attempt to recreate that state.
// Specifically, it will run the given apps tagged with the according processID
type RestoreInfo map[node.ProcessID]*node.AppInstallation

// ProcessManager is a way to manage processes across many different drivers / runtimes
// Right now, all it does is hold a set of drivers and pass through to calls to the Driver interface for each of them
// For that reason, we could consider removing it in the future and simply holding a map[node.Driver]Driver in the caller to this
type ProcessManager interface {
	// ListAllProcesses returns a list of all running process IDs, across all drivers
	ListRunningProcesses(context.Context) ([]node.ProcessID, error)
	// StartProcess starts a process for the given app installation with the given process ID
	// It is expected that the driver can be derived from AppInstallation
	StartProcess(context.Context, node.ProcessID, *node.AppInstallation) error
	// StopProcess stops the process corresponding to the given process ID
	StopProcess(context.Context, node.ProcessID) error
	// Returns process state, true if exists, otherwise nil, false to indicate non-existence
	IsRunning(context.Context, node.ProcessID) (bool, node.Driver, error)
	// ProcessManager should implement Component -- specifically, restore state given by RestoreInfo
	node.Component[RestoreInfo]
}

var (
	ErrDriverNotFound = errors.New("no driver found")
)

type baseProcessManager struct {
	drivers map[node.Driver]Driver
}

func NewProcessManager(drivers []Driver) ProcessManager {
	pm := &baseProcessManager{
		drivers: make(map[node.Driver]Driver),
	}
	for _, driver := range drivers {
		pm.drivers[driver.Type()] = driver
	}
	return pm
}

func (pm *baseProcessManager) ListRunningProcesses(ctx context.Context) ([]node.ProcessID, error) {
	var allProcs []node.ProcessID
	for _, driver := range pm.drivers {
		procs, err := driver.ListRunningProcesses(ctx)
		if err != nil {
			return nil, err
		}
		allProcs = append(allProcs, procs...)
	}
	return allProcs, nil
}

func (pm *baseProcessManager) IsRunning(ctx context.Context, id node.ProcessID) (bool, node.Driver, error) {
	var derr error
	for _, driver := range pm.drivers {
		ok, err := driver.IsRunning(ctx, id)
		if ok && err == nil {
			fmt.Println("found!", ok, driver.Type())
			// found process -- early return
			return true, driver.Type(), nil
		} else if err != nil {
			// set result error if driver returned error
			derr = err
		}
	}
	if derr != nil {
		return false, node.DriverUnknown, derr
	}
	return false, node.DriverUnknown, nil
}

func (pm *baseProcessManager) StartProcess(ctx context.Context, id node.ProcessID, app *node.AppInstallation) error {
	driver, ok := pm.drivers[app.Driver]
	if !ok {
		return fmt.Errorf("%w: %s", ErrDriverNotFound, app.Driver)
	}
	ok, err := driver.IsRunning(ctx, id)
	if ok && err == nil {
		return fmt.Errorf("%w: %s", ErrProcessAlreadyRunning, id)
	} else if err != nil {
		return err
	}

	return driver.StartProcess(ctx, id, app)
}

// This could be a bit more efficient if the process manager new which driver the process belonged to
// However, it's possible to get into a state where either the ndoe state does not contain the process and it is running
// For this case, blindly pass the signal to all drivers; processes should be unique across drivers, so this is OK.
func (pm *baseProcessManager) StopProcess(ctx context.Context, processID node.ProcessID) error {
	for _, driver := range pm.drivers {
		err := driver.StopProcess(ctx, processID)
		if !errors.Is(err, ErrNoProcFound) {
			return err
		}
	}
	return nil
}

func (pm *baseProcessManager) SupportedTransitionTypes() []hdb.TransitionType {
	return []hdb.TransitionType{
		hdb.TransitionStartProcess,
		hdb.TransitionStopProcess,
	}
}

func (pm *baseProcessManager) RestoreFromState(ctx context.Context, state RestoreInfo) error {
	for id, app := range state {
		err := pm.StartProcess(ctx, id, app)
		if err != nil {
			return fmt.Errorf("Error starting process %s: %s", id, err)
		}
	}
	return nil
}
