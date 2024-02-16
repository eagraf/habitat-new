package processes

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

type StartProcessExecutor struct {
	processManager ProcessManager
}

func (e *StartProcessExecutor) TransitionType() string {
	return node.TransitionStartProcess
}

func (e *StartProcessExecutor) ShouldExecute(update *hdb.StateUpdate) (bool, error) {
	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition, &processStartTransition)
	if err != nil {
		return false, err
	}

	_, err = e.processManager.GetProcess(processStartTransition.Process.ID)
	if err != nil {
		return true, nil
	}

	return false, nil
}

func (e *StartProcessExecutor) Execute(update *hdb.StateUpdate) error {
	var processStartTransition node.ProcessStartTransition
	err := json.Unmarshal(update.Transition, &processStartTransition)
	if err != nil {
		return err
	}

	return e.processManager.StartProcess(processStartTransition.Process, processStartTransition.App)
}

func (e *StartProcessExecutor) PostHook(update *hdb.StateUpdate) error {
	return nil
}
