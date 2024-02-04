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
				"certificate": { "type": "string" },
				"app_installations": {
					"type": "array",
					"items": {
						"$ref": "#/$defs/app_installation"
					}
				}
			},
			"required": [ "id", "username", "certificate" ]
		},
		"app_installation": {
			"type": "object",
			"properties": {
				"name": { "type": "string" },
				"version": { "type": "string" },
				"driver": { 
					"type": "string",
					"enum": [ "docker" ]
				},
				"registry_url_base": { "type": "string" },
				"registry_app_id": { "type": "string" },
				"registry_tag": { "type": "string" },
				"state": {
					"type": "string",
					"enum": [ "installing", "installed", "uninstalled" ]
				}
			},
			"required": [ "name", "version", "driver", "registry_url_base", "registry_app_id", "registry_tag", "state" ]
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
	Certificate string  `json:"certificate"` // TODO turn this into b64
	Users       []*User `json:"users"`
}

type User struct {
	ID               string                  `json:"id"`
	Username         string                  `json:"username"`
	Certificate      string                  `json:"certificate"` // TODO turn this into b64
	AppInstallations []*AppInstallationState `json:"app_installations"`
}

const AppStateInstalling = "installing"
const AppStateInstalled = "installed"
const AppStateUninstalled = "uninstalled"

type AppInstallationState struct {
	*AppInstallation
	State string `json:"state"`
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
	var is *NodeState
	err := json.Unmarshal(initState, &is)
	if err != nil {
		return nil, err
	}
	return &InitalizationTransition{
		InitState: is,
	}, nil
}
