package controller

import (
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/rs/zerolog/log"
)

type ProcessRestorer struct {
	processManager process.ProcessManager
	nodeController NodeController
}

// TODO: Return []error for each failure rather than one
func (r *ProcessRestorer) Restore(restoreEvent hdb.StateUpdate) error {
	nodeState := restoreEvent.NewState().(*node.State)
	for _, process := range nodeState.Processes {
		app, err := nodeState.GetAppByID(process.AppID)
		if err != nil {
			log.Error().Msgf("Error getting app %s: %s", process.AppID, err)
			return err
		}

		switch process.State {
		case node.ProcessStateStarted:
			err = r.processManager.StartProcess(process.Process, app.AppInstallation)
			if err != nil {
				log.Error().Msgf("Error starting process %s: %s", process.ID, err)
				return err
			}
		}
	}
	return nil
}
