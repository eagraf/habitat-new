package schemas

import (
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/user"
)

func GetSchema(schemaType string) (state.Schema, error) {
	switch schemaType {
	case node.SchemaName:
		return &node.NodeSchema{}, nil
	case user.SchemaName:
		return &user.UserSchema{}, nil
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}
}

// TODO this should account for schema version too
func StateMachineFactory(schemaType string, initState []byte, replicator state.Replicator, executor state.Executor) (state.StateMachineController, error) {
	var schema state.Schema

	switch schemaType {
	case node.SchemaName:
		schema = &node.NodeSchema{}
	case user.SchemaName:
		schema = &user.UserSchema{}
	default:
		return nil, fmt.Errorf("schema type %s not found", schemaType)
	}

	if initState == nil {
		initStateStruct, err := schema.InitState()
		if err != nil {
			return nil, err
		}
		initStateBytes, err := initStateStruct.Bytes()
		initState = initStateBytes
	}

	stateMachineController, err := state.NewStateMachine(schema.Bytes(), initState, replicator, executor)
	if err != nil {
		return nil, err
	}
	return stateMachineController, nil
}