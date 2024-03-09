package node

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/qri-io/jsonschema"
)

const SchemaName = "node"
const CurrentVersion = "v0.0.1"
const LatestVersion = "v0.0.3"

//go:embed schema/schema.json
var nodeSchemaRaw string

// TODO structs defined here can embed the immutable structs, but also include mutable fields.

type NodeState struct {
	NodeID           string                           `json:"node_id"`
	Name             string                           `json:"name"`
	Certificate      string                           `json:"certificate"` // TODO turn this into b64
	SchemaVersion    string                           `json:"schema_version"`
	TestField        string                           `json:"test_field,omitempty"`
	Users            map[string]*User                 `json:"users"`
	Processes        map[string]*ProcessState         `json:"processes"`
	AppInstallations map[string]*AppInstallationState `json:"app_installations"`
}

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"` // TODO turn this into b64
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

func (s NodeState) GetAppsForUser(userID string) ([]*AppInstallationState, error) {
	apps := make([]*AppInstallationState, 0)
	for _, app := range s.AppInstallations {
		if app.UserID == userID {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

func (s NodeState) GetProcessesForUser(userID string) ([]*ProcessState, error) {
	procs := make([]*ProcessState, 0)
	for _, proc := range s.Processes {
		if proc.UserID == userID {
			procs = append(procs, proc)
		}
	}
	return procs, nil
}

func (s NodeState) Copy() (*NodeState, error) {
	marshaled, err := s.Bytes()
	if err != nil {
		return nil, err
	}
	var copy NodeState
	err = json.Unmarshal(marshaled, &copy)
	if err != nil {
		return nil, err
	}
	return &copy, nil
}

type NodeSchema struct {
	schemaBytes []byte
}

func (s *NodeSchema) Name() string {
	return SchemaName
}

func (s *NodeSchema) ID(version string) string {
	return fmt.Sprintf("https://github.com/eagraf/habitat-new/archive/refs/tags/%s-%s.json", SchemaName, CurrentVersion)
}

func (s *NodeSchema) InitState() (hdb.State, error) {
	return &NodeState{
		Users:            make(map[string]*User),
		Processes:        make(map[string]*ProcessState),
		AppInstallations: make(map[string]*AppInstallationState),
	}, nil
}

func (s *NodeSchema) Bytes() []byte {
	if s.schemaBytes != nil {
		return s.schemaBytes
	}
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
	is.SchemaVersion = CurrentVersion
	return &InitalizationTransition{
		InitState: is,
	}, nil
}

func (s *NodeSchema) JSONSchema() *jsonschema.Schema {
	var schema jsonschema.Schema
	err := json.Unmarshal(s.Bytes(), &schema)
	if err != nil {
		panic(err)
	}
	return &schema
}

func (s *NodeSchema) ValidateState(state []byte) error {
	keyErrs, err := s.JSONSchema().ValidateBytes(context.Background(), state)
	if err != nil {
		return err
	}
	if len(keyErrs) > 0 {
		return fmt.Errorf("validation failed: %v", keyErrs)
	}
	return nil
}
