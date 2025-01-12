package controller

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/process"
)

type controller2 struct {
	db             hdb.Client
	processManager process.ProcessManager
}

func newController2(pm process.ProcessManager) (*controller2, error) {
	_, ok := pm.(Component[*node.Process])
	if !ok {
		return nil, fmt.Errorf("Process manager of type %T does not implement Component[*node.Process]", pm)
	}
	return &controller2{
		processManager: pm,
	}, nil
}

func (c *controller2) getNodeState() (node.State, error) {
	var nodeState node.State
	err := json.Unmarshal(c.db.Bytes(), &nodeState)
	if err != nil {
		return node.State{}, err
	}
	return nodeState, nil
}

func (c *controller2) startProcess(installationID string) error {
	state, err := c.getNodeState()
	if err != nil {
		return err
	}
	app, ok := state.AppInstallations[installationID]
	if !ok {
		return fmt.Errorf("app with ID %s not found", installationID)
	}

	transition := &node.ProcessStartTransition{
		AppID: installationID,
	}

	bytes, err := state.Bytes()
	if err != nil {
		return err
	}
	err = transition.Enrich(bytes)
	if err != nil {
		return err
	}

	_, err = c.db.ProposeTransitionsEnriched([]hdb.Transition{
		transition,
	})
	if err != nil {
		return err
	}

	proc := transition.EnrichedData.Process
	err = c.processManager.StartProcess(proc.Process, app.AppInstallation)
	if err != nil {
		return err
	}

	_, err = c.db.ProposeTransitionsEnriched([]hdb.Transition{
		&node.ProcessRunningTransition{
			ProcessID: proc.ID,
		},
	})
	if err != nil {
		return err
	}
	return nil
}
