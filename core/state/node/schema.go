package node

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/eagraf/habitat-new/internal/node/hdb"
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
						"type": "string"
					}
				}
			},
			"required": [ "id", "username", "certificate", "app_installations" ]
		},
		"app_installation": {
			"type": "object",
			"properties": {
				"id": { "type": "string" },
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
			"required": [ "id", "name", "version", "driver", "registry_url_base", "registry_app_id", "registry_tag", "state" ]
		},
		"process": {
			"type": "object",
			"properties": {
				"id": {"type": "string"},
				"app_id": { "type": "string" },
				"user_id": { "type": "string" },
				"created": { "type": "string" },
				"state": { "type": "string" },
				"ext_driver_id": { "type": "string" }
			},
			"required": [ "id", "app_id", "ext_driver_id", "user_id", "state", "created" ]
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
			"type": "object",
			"properties": {
				"itemType": { "$ref": "#/$defs/user" }
			}
		},
		"app_installations": {
			"type": "object",
			"properties": {
				"itemType": { "$ref": "#/$defs/app_installation" }
			}
		},
		"processes": {
			"type": "object",
			"properties": {
				"itemType": { "$ref": "#/$defs/process" }
			}
		}
	},
	"required": [ "node_id", "name", "certificate", "users" ]
}`

// TODO structs defined here can embed the immutable structs, but also include mutable fields.

type NodeState struct {
	NodeID           string                           `json:"node_id"`
	Name             string                           `json:"name"`
	Certificate      string                           `json:"certificate"` // TODO turn this into b64
	Users            map[string]*User                 `json:"users"`
	Processes        map[string]*ProcessState         `json:"processes"`
	AppInstallations map[string]*AppInstallationState `json:"app_installations"`
}

type User struct {
	ID               string   `json:"id"`
	Username         string   `json:"username"`
	Certificate      string   `json:"certificate"` // TODO turn this into b64
	AppInstallations []string `json:"app_installations"`
}

const AppStateInstalling = "installing"
const AppStateInstalled = "installed"
const AppStateUninstalled = "uninstalled"

type AppInstallationState struct {
	*AppInstallation
	State string `json:"state"`
}

type ProcessState struct {
	*Process
	State       string `json:"state"`
	ExtDriverID string `json:"ext_driver_id"`
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

func (s NodeState) GetAppByID(appID string) (*AppInstallationState, error) {
	for _, app := range s.AppInstallations {
		if app.ID == appID {
			return app, nil
		}
	}
	return nil, fmt.Errorf("app with ID %s not found", appID)
}

type NodeSchema struct {
}

func (s *NodeSchema) Name() string {
	return SchemaName
}

func (s *NodeSchema) InitState() (hdb.State, error) {
	return &NodeState{
		Users:            make(map[string]*User),
		Processes:        make(map[string]*ProcessState),
		AppInstallations: make(map[string]*AppInstallationState),
	}, nil
}

func (s *NodeSchema) Bytes() []byte {
	return []byte(nodeSchemaRaw)
}

func (s *NodeSchema) Type() reflect.Type {
	return reflect.TypeOf(&NodeState{})
}

func (s *NodeSchema) InitializationTransition(initState []byte) (hdb.Transition, error) {
	var is *NodeState
	err := json.Unmarshal(initState, &is)
	if err != nil {
		return nil, err
	}
	return &InitalizationTransition{
		InitState: is,
	}, nil
}
