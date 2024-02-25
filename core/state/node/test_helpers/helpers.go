package test_helpers

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

func StateUpdateTestHelper(transition hdb.Transition, newState *node.NodeState) (*hdb.StateUpdate, error) {
	transBytes, err := json.Marshal(transition)
	if err != nil {
		return nil, err
	}

	stateBytes, err := json.Marshal(newState)
	if err != nil {
		return nil, err
	}

	return &hdb.StateUpdate{
		SchemaType:     node.SchemaName,
		Transition:     transBytes,
		TransitionType: transition.Type(),
		NewState:       stateBytes,
	}, nil
}
