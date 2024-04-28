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
const CurrentVersion = "v0.0.4"
const LatestVersion = "v0.0.4"

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

func (s NodeState) Schema() hdb.Schema {
	ns := &NodeSchema{}
	return ns
}

func (s NodeState) Bytes() ([]byte, error) {
	return json.Marshal(s)
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

func (s NodeState) Validate() error {
	schemaVersion := s.SchemaVersion

	ns := &NodeSchema{}
	jsonSchema, err := ns.JSONSchemaForVersion(schemaVersion)
	if err != nil {
		return err
	}
	stateBytes, err := s.Bytes()
	if err != nil {
		return err
	}
	keyErrs, err := jsonSchema.ValidateBytes(context.Background(), stateBytes)
	if err != nil {
		return err
	}

	if len(keyErrs) > 0 {
		return fmt.Errorf("validation failed: %v", keyErrs)
	}
	return nil
}

type NodeSchema struct{}

func (s *NodeSchema) Name() string {
	return SchemaName
}

func (s *NodeSchema) InitState() (hdb.State, error) {
	return &NodeState{
		SchemaVersion:    CurrentVersion,
		Users:            make(map[string]*User),
		Processes:        make(map[string]*ProcessState),
		AppInstallations: make(map[string]*AppInstallationState),
	}, nil
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

func (s *NodeSchema) JSONSchemaForVersion(version string) (*jsonschema.Schema, error) {

	migrations, err := readSchemaMigrationFiles()
	if err != nil {
		return nil, err
	}

	schema, err := getSchemaVersion(migrations, version)
	if err != nil {
		return nil, err
	}

	rs := &jsonschema.Schema{}
	err = json.Unmarshal([]byte(schema), rs)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %s", err)
	}

	return rs, nil
}

func (s *NodeSchema) ValidateState(state []byte) error {
	var stateObj NodeState
	err := json.Unmarshal(state, &stateObj)
	if err != nil {
		return err
	}

	version := stateObj.SchemaVersion

	jsonSchema, err := s.JSONSchemaForVersion(version)
	if err != nil {
		return err
	}

	keyErrs, err := jsonSchema.ValidateBytes(context.Background(), state)
	if err != nil {
		return err
	}
	if len(keyErrs) > 0 {
		return fmt.Errorf("validation failed: %v", keyErrs)
	}
	return nil
}
