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

func (e *StartProcessExecutor) ShouldExecute(update hdb.StateUpdateInternal) (bool, error) {
	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition(), &processStartTransition)
	if err != nil {
		return false, err
	}

	_, err = e.processManager.GetProcess(processStartTransition.Process.ID)
	if err != nil {
		return true, nil
	}

	return false, nil
}

func (e *StartProcessExecutor) Execute(update hdb.StateUpdateInternal) error {
	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition(), &processStartTransition)
	if err != nil {
		return err
	}

	err = e.processManager.StartProcess(processStartTransition.Process, processStartTransition.App)
	if err != nil {
		return err
	}

	err = e.nodeController.SetProcessRunning(processStartTransition.Process.ID)
	if err != nil {
		return err
	}

	return nil
}

func (e *StartProcessExecutor) PostHook(update hdb.StateUpdateInternal) error {
	return nil
}
