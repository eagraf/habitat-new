package node

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	TransitionInitialize          = "initialize"
	TransitionAddUser             = "add_user"
	TransitionStartInstallation   = "start_installation"
	TransitionFinishInstallation  = "finish_installation"
	TransitionStartUninstallation = "start_uninstallation"
	TransitionStartProcess        = "process_start"
	TransitionProcessRunning      = "process_running"
	TransitionStopProcess         = "process_stop"
)

type InitalizationTransition struct {
	InitState *NodeState `json:"init_state"`
}

func (t *InitalizationTransition) Type() string {
	return TransitionInitialize
}

func (t *InitalizationTransition) Patch(oldState []byte) ([]byte, error) {
	if t.InitState.Users == nil {
		t.InitState.Users = make(map[string]*User, 0)
	}

	if t.InitState.AppInstallations == nil {
		t.InitState.AppInstallations = make(map[string]*AppInstallationState)
	}

	if t.InitState.Processes == nil {
		t.InitState.Processes = make(map[string]*ProcessState)
	}

	marshaled, err := json.Marshal(t.InitState)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`[{
		"op": "add",
		"path": "",
		"value": %s
	}]`, marshaled)), nil
}

func (t *InitalizationTransition) Validate(oldState []byte) error {
	if t.InitState == nil {
		return fmt.Errorf("init state cannot be nil")
	}
	return nil
}

type AddUserTransition struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
}

func (t *AddUserTransition) Type() string {
	return TransitionAddUser
}

func (t *AddUserTransition) Patch(oldState []byte) ([]byte, error) {
	return []byte(fmt.Sprintf(`[{
		"op": "add",
		"path": "/users/%s",
		"value": {
			"id": "%s",
			"username": "%s",
			"certificate": "%s",
			"app_installations": []
		}
	}]`, t.UserID, t.UserID, t.Username, t.Certificate)), nil
}

func (t *AddUserTransition) Validate(oldState []byte) error {

	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	_, ok := oldNode.Users[t.UserID]
	if ok {
		return fmt.Errorf("user with id %s already exists", t.UserID)
	}

	// Check for conflicting usernames
	for _, user := range oldNode.Users {
		if user.Username == t.Username {
			return fmt.Errorf("user with username %s already exists", t.Username)
		}
	}
	return nil
}

type StartInstallationTransition struct {
	UserID string `json:"user_id"`
	*AppInstallation
}

func (t *StartInstallationTransition) Type() string {
	return TransitionStartInstallation
}

func (t *StartInstallationTransition) Patch(oldState []byte) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	appInstallation := t.AppInstallation
	appInstallation.ID = uuid.New().String()

	appState := &AppInstallationState{
		AppInstallation: t.AppInstallation,
		State:           AppLifecycleStateInstalling,
	}
	marshalledApp, err := json.Marshal(appState)
	if err != nil {
		return nil, err
	}

	_, ok := oldNode.Users[t.UserID]
	if !ok {
		return nil, fmt.Errorf("user with id %s not found", t.UserID)
	}

	return []byte(fmt.Sprintf(`[
		{
			"op": "add",
			"path": "/app_installations/%s",
			"value": %s
		},
		{
			"op" : "add",
			"path": "/users/%s/app_installations/-",
			"value": "%s"
		}
	]`, t.AppInstallation.ID, string(marshalledApp), t.AppInstallation.UserID, t.AppInstallation.ID)), nil
}

func (t *StartInstallationTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	_, ok := oldNode.Users[t.UserID]
	if !ok {
		return fmt.Errorf("user with id %s not found", t.UserID)
	}

	app, ok := oldNode.AppInstallations[t.AppInstallation.ID]
	if ok {
		if app.Version == t.Version {
			return fmt.Errorf("app %s version %s for user %s found in state %s", t.Name, t.Version, t.UserID, app.State)
		} else {
			// TODO eventually this will be part of an upgrade flow
			return fmt.Errorf("app %s for user %s found in state with different version %s", t.Name, t.UserID, app.Version)
		}
	}

	// Look for matching registry URL and package ID
	for _, app := range oldNode.AppInstallations {
		if app.RegistryURLBase == t.RegistryURLBase && app.RegistryPackageID == t.RegistryPackageID {
			return fmt.Errorf("app %s for user %s found in state with different version %s", app.Name, t.UserID, app.Version)
		}
	}

	return nil
}

type FinishInstallationTransition struct {
	UserID          string `json:"user_id"`
	AppID           string `json:"app_id"`
	RegistryURLBase string `json:"registry_url_base"`
	RegistryAppID   string `json:"registry_app_id"`
}

func (t *FinishInstallationTransition) Type() string {
	return TransitionFinishInstallation
}

func (t *FinishInstallationTransition) Patch(oldState []byte) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`[{
		"op": "replace",
		"path": "/app_installations/%s/state",
		"value": "%s"
	}]`, t.AppID, AppLifecycleStateInstalled)), nil
}

func (t *FinishInstallationTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	app, ok := oldNode.AppInstallations[t.AppID]
	if !ok {
		return fmt.Errorf("app with id %s not found", t.AppID)
	}

	_, ok = oldNode.Users[t.UserID]
	if !ok {
		return fmt.Errorf("user with id %s not found", t.UserID)
	}

	if app.RegistryURLBase != t.RegistryURLBase || app.RegistryPackageID != t.RegistryAppID {
		return fmt.Errorf("app %s for user %s found in state with different registry url %s and package id %s", app.Name, t.UserID, app.RegistryURLBase, app.RegistryPackageID)
	}

	if app.State != "installing" {
		return fmt.Errorf("app %s for user %s is in state %s", app.Name, t.UserID, app.State)
	}

	return nil
}

// TODO handle uninstallation

type ProcessStartTransition struct {
	*Process
	App *AppInstallation `json:"app"`
}

func (t *ProcessStartTransition) Type() string {
	return TransitionStartProcess
}

func (t *ProcessStartTransition) Patch(oldState []byte) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	proc := ProcessState{
		Process:     t.Process,
		State:       ProcessStateStarting,
		ExtDriverID: "", // this should not be set yet
	}

	proc.Process.Created = time.Now().Format(time.RFC3339)

	marshaled, err := json.Marshal(proc)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`[{
			"op": "add",
			"path": "/processes/%s",
			"value": %s
		}]`, t.ID, marshaled)), nil
}

func (t *ProcessStartTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	if t.Process == nil {
		return fmt.Errorf("Process cannot be nil")
	}

	// Make sure the app installation exists
	app, ok := oldNode.AppInstallations[t.App.ID]
	if !ok {
		return fmt.Errorf("app with id %s does not exist", t.App.ID)
	}
	if app.State != AppLifecycleStateInstalled {
		return fmt.Errorf("app with id %s for user %s is not in state %s", t.AppID, t.UserID, AppLifecycleStateInstalled)
	}

	// Check user exists
	_, ok = oldNode.Users[t.UserID]
	if !ok {
		return fmt.Errorf("user with id %s does not exist", t.UserID)
	}

	for procID, proc := range oldNode.Processes {
		// Make sure that no process exists with the same ID
		if procID == t.ID {
			return fmt.Errorf("Process with id %s already exists", t.ID)
		}
		// Make sure that no app with the same ID has a process
		if proc.AppID == t.AppID {
			return fmt.Errorf("app with id %s already has a process", t.AppID)
		}
	}

	return nil
}

type ProcessRunningTransition struct {
	ProcessID string `json:"process_id"`
}

func (t *ProcessRunningTransition) Type() string {
	return TransitionProcessRunning
}

func (t *ProcessRunningTransition) Patch(oldState []byte) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	// Find the matching process
	proc, ok := oldNode.Processes[t.ProcessID]
	if !ok {
		return nil, fmt.Errorf("process with id %s not found", t.ProcessID)
	}
	proc.State = ProcessStateRunning

	marshaled, err := json.Marshal(proc)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`[{
		"op": "replace",
		"path": "/processes/%s",
		"value": %s
	}]`, t.ProcessID, marshaled)), nil
}

func (t *ProcessRunningTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	// Make sure there is a matching process
	proc, ok := oldNode.Processes[t.ProcessID]
	if !ok {
		return fmt.Errorf("process with id %s not found", t.ProcessID)
	}
	if proc.State != ProcessStateStarting {
		return fmt.Errorf("Process with id %s is in state %s, must be in state %s", t.ProcessID, proc.State, ProcessStateStarting)
	}

	return nil
}

type ProcessStopTransition struct {
	ProcessID string `json:"process_id"`
}

func (t *ProcessStopTransition) Type() string {
	return TransitionStopProcess
}

func (t *ProcessStopTransition) Patch(oldState []byte) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return nil, err
	}

	_, ok := oldNode.Processes[t.ProcessID]
	if !ok {
		return nil, fmt.Errorf("process with id %s not found", t.ProcessID)
	}

	return []byte(fmt.Sprintf(`[{
		"op": "remove",
		"path": "/processes/%s"
	}]`, t.ProcessID)), nil
}

func (t *ProcessStopTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	// Make sure there is a matching process
	_, ok := oldNode.Processes[t.ProcessID]
	if !ok {
		return fmt.Errorf("process with id %s not found", t.ProcessID)
	}
	return nil
}
