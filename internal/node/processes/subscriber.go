package processes

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

type StartProcessExecutor struct {
	processManager ProcessManager
	nodeController controller.NodeController
}

func (e *StartProcessExecutor) TransitionType() string {
	return node.TransitionStartProcess
}

func (e *StartProcessExecutor) ShouldExecute(update hdb.StateUpdate) (bool, error) {
	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition(), &processStartTransition)
	if err != nil {
		return false, err
	}

	// If somehow this process is already running, we don't need to start it again
	// This can happen if the node restarts while a process is starting
	_, err = e.processManager.IsProcessRunning(processStartTransition.EnrichedData.Process.ID)
	if err != nil {
		return true, nil
	}

	return false, nil
}

func (e *StartProcessExecutor) Execute(update hdb.StateUpdate) error {
	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition(), &processStartTransition)
	if err != nil {
		return err
	}

	nodeState := update.NewState().(*node.State)

	app, err := nodeState.GetAppByID(processStartTransition.AppID)
	if err != nil {
		return err
	}

	extProcessID, err := e.processManager.StartProcess(processStartTransition.EnrichedData.Process.Process, app.AppInstallation)
	if err != nil {
		return err
	}

	err = e.nodeController.SetProcessRunning(
		processStartTransition.EnrichedData.Process.ID,
		extProcessID,
	)
	if err != nil {
		return err
	}

	return nil
}

func (e *StartProcessExecutor) PostHook(update hdb.StateUpdate) error {
	return nil
}

type StopProcessExecutor struct {
	processManager ProcessManager
	nodeController controller.NodeController
}

func NewStopProcessExecutor(processManager ProcessManager, nodeController controller.NodeController) *StopProcessExecutor {
	return &StopProcessExecutor{
		processManager: processManager,
		nodeController: nodeController,
	}
}

func (e *StopProcessExecutor) TransitionType() string {
	return node.TransitionStopProcess
}

func (e *StopProcessExecutor) ShouldExecute(update hdb.StateUpdate) (bool, error) {
	var processStopTransition node.ProcessStopTransition
	err := json.Unmarshal(update.Transition(), &processStopTransition)
	if err != nil {
		return false, err
	}

	_, err = e.processManager.GetProcess(processStopTransition.ProcessID)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (e *StopProcessExecutor) Execute(update hdb.StateUpdate) error {
	var processStopTransition node.ProcessStopTransition
	err := json.Unmarshal(update.Transition(), &processStopTransition)
	if err != nil {
		return err
	}

	err = e.processManager.StopProcess(processStopTransition.ProcessID)
	if err != nil {
		return err
	}

	return nil
}

// PostHook is called even if ShouldExecute returns false.
func (e *StopProcessExecutor) PostHook(update hdb.StateUpdate) error {
	var processStopTransition node.ProcessStopTransition
	err := json.Unmarshal(update.Transition(), &processStopTransition)
	if err != nil {
		return err
	}

	err = e.nodeController.FinishProcessStop(processStopTransition.ProcessID)
	if err != nil {
		return err
	}

	return nil
}
