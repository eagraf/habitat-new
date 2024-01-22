package user

import (
	"encoding/json"
	"reflect"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
)

const SchemaName = "user"

var userSchemaRaw = `
{
	"$defs": {
		"node": {
			"type": "object",
			"properties": {
				"id": { "type": "string" },
				"address": { "type": "string" },
				"certificate": { "type": "string" }
			},
			"required": [ "id", "address", "certificate" ]
		}
	},
	"title": "Habitat User State",
	"type": "object",
	"properties": {
		"user_id": {
			"type": "string"
		},
		"username": {
			"type": "string"
		},
		"public_key": {
			"type": "string"
		},
		"nodes": {
			"type": "array",
			"items": {
				"$ref": "#/$defs/node"
			}
		}
	},
	"required": [ "user_id", "username", "public_key", "nodes" ]
}`

type UserState struct {
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	PublicKey string  `json:"public_key"`
	Nodes     []*Node `json:"nodes"`
}

type Node struct {
	ID          string `json:"id"`
	Address     string `json:"address"`
	Certificate string `json:"certificate"`
}

func (s UserState) Schema() []byte {
	return []byte(userSchemaRaw)
}

func (s UserState) Bytes() ([]byte, error) {
	marshaled, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return marshaled, nil
}

type UserSchema struct {
}

func (s *UserSchema) Name() string {
	return SchemaName
}

func (s *UserSchema) InitState() (state.State, error) {
	return &UserState{
		Nodes: make([]*Node, 0),
	}, nil
}

func (s *UserSchema) Bytes() []byte {
	return []byte(userSchemaRaw)
}

func (s *UserSchema) Type() reflect.Type {
	return reflect.TypeOf(&UserState{})
}

func (s *UserSchema) InitializationTransition(initState []byte) (state.Transition, error) {
	var t InitalizationTransition
	err := json.Unmarshal(initState, &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
