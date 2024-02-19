package processes

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/rs/zerolog/log"
)

type ProcessRestorer struct {
	processManager ProcessManager
}

func (r *ProcessRestorer) Restore(restoreEvent *hdb.StateUpdate) error {
	var nodeState node.NodeState
	err := json.Unmarshal(restoreEvent.NewState, &nodeState)
	if err != nil {
		return err
	}

	for _, process := range nodeState.Processes {
		if _, err := r.processManager.GetProcess(process.ID); err != nil {
			continue
		}
		if process.State == node.ProcessStateRunning {
			app, err := nodeState.GetAppByID(process.AppID)
			if err != nil {
				log.Error().Msgf("Error getting app %s: %s", process.AppID, err)
				continue
			}
			err = r.processManager.StartProcess(process.Process, app.AppInstallation)
			if err != nil {
				log.Error().Msgf("Error starting process %s: %s", process.ID, err)
				continue
			}
		}
	}

	return nil
}
