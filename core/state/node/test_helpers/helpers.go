package test_helpers

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
)

func StateUpdateTestHelper(transition hdb.Transition, newState *node.NodeState) (*hdb.StateUpdate, error) {
	stateBytes, err := newState.Bytes()
	if err != nil {
		return nil, err
	}
	err = transition.Enrich(stateBytes)
	transBytes, err := json.Marshal(transition)
	if err != nil {
		return nil, err
	}

	updateInternal := node.NewNodeStateUpdateInternal(newState, &hdb.TransitionWrapper{
		Transition: transBytes,
		Type:       transition.Type(),
	})

	return &hdb.StateUpdate{
		SchemaType:          node.SchemaName,
		StateUpdateInternal: updateInternal,
	}, nil
}
