package schemas

import (
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/core"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
)

func GetSchema(schemaType string) (core.Schema, error) {
	switch schemaType {
	case node.SchemaName:
		return &node.NodeSchema{}, nil
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}
}

// TODO this should account for schema version too
func StateMachineFactory(databaseID string, schemaType string, initState []byte, replicator state.Replicator, publisher pubsub.Publisher[state.StateUpdate]) (state.StateMachineController, error) {
	var schema core.Schema

	switch schemaType {
	case node.SchemaName:
		schema = &node.NodeSchema{}
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}

	if initState == nil {
		initStateStruct, err := schema.InitState()
		if err != nil {
			return nil, err
		}
		initStateBytes, err := initStateStruct.Bytes()
		if err != nil {
			return nil, err
		}
		initState = initStateBytes
	}

	stateMachineController, err := state.NewStateMachine(databaseID, schema.Bytes(), initState, replicator, publisher)
	if err != nil {
		return nil, err
	}
	return stateMachineController, nil
}
