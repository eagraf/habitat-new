package state

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/core/state/node/test_helpers"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/pubsub"
)

func GetSchema(schemaType string) (hdb.Schema, error) {
	switch schemaType {
	case node.SchemaName:
		return &node.NodeSchema{}, nil
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}
}

// TODO this should account for schema version too
func StateMachineFactory(databaseID string, schemaType string, emptyState []byte, publisher pubsub.Publisher[hdb.StateUpdate], writeToDB func([]byte) error) (StateMachineController, error) {
	var schema hdb.Schema

	switch schemaType {
	case node.SchemaName:
		schema = &node.NodeSchema{}
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}

	if emptyState == nil {
		initStateStruct, err := schema.EmptyState()
		if err != nil {
			return nil, err
		}
		emptyStateBytes, err := initStateStruct.Bytes()
		if err != nil {
			return nil, err
		}
		emptyState = emptyStateBytes
	}

	stateMachineController, err := NewStateMachine(databaseID, schema, emptyState, publisher, writeToDB)
	if err != nil {
		return nil, err
	}
	return stateMachineController, nil
}

func StateUpdateInternalFactory(
	schemaType string,
	stateBytes []byte,
	transitionWrapper *hdb.TransitionWrapper,
	metadata *hdb.StateUpdateMetadata,
) (hdb.StateUpdate, error) {
	switch schemaType {
	case node.SchemaName:
		var state node.State
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			return nil, err
		}
		return test_helpers.NewNodeStateUpdate(&state, transitionWrapper, metadata), nil
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}
}
