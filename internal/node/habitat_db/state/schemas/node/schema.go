package node

import (
	"encoding/json"
	"reflect"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
)

const SchemaName = "node"

var nodeSchemaRaw = `
{
	"$defs": {
		"user": {
			"type": "object",
			"properties": {
				"id": { "type": "string" },
				"username": { "type": "string" },
				"public_key": { "type": "string" }
			}
		}
	},
	"title": "Habitat Node State",
	"type": "object",
	"properties": {
		"node_id": {
			"type": "string"
		},
		"name": {
			"type": "string"
		},
		"certificate": {
			"type": "string"
		},
		"users": {
			"type": "array",
			"items": {
				"$ref": "#/$defs/user"
			}
		}
	},
	"required": [ "node_id", "name", "certificate", "users" ]
}`

type NodeState struct {
	NodeID      string  `json:"node_id"`
	Name        string  `json:"name"`
	Certificate string  `json:"certificate"`
	Users       []*User `json:"users"`
}

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
}

func (s NodeState) Schema() []byte {
	return []byte(nodeSchemaRaw)
}

func (s NodeState) Bytes() ([]byte, error) {
	marshaled, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return marshaled, nil
}

type NodeSchema struct {
}

func (s *NodeSchema) Name() string {
	return SchemaName
}

func (s *NodeSchema) InitState() (state.State, error) {
	return &NodeState{
		Users: make([]*User, 0),
	}, nil
}

func (s *NodeSchema) Bytes() []byte {
	return []byte(nodeSchemaRaw)
}

func (s *NodeSchema) Type() reflect.Type {
	return reflect.TypeOf(&NodeState{})
}

func (s *NodeSchema) InitializationTransition(initState []byte) (state.Transition, error) {
	var t InitalizationTransition
	err := json.Unmarshal(initState, &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
