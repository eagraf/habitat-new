package node

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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
		t.InitState.Users = make([]*User, 0)
	}

	if t.InitState.Processes == nil {
		t.InitState.Processes = make([]*ProcessState, 0)
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
		"path": "/users/-",
		"value": {
			"id": "%s",
			"username": "%s",
			"certificate": "%s",
			"app_installations": []
		}
	}]`, t.UserID, t.Username, t.Certificate)), nil
}

func (t *AddUserTransition) Validate(oldState []byte) error {

	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	for _, user := range oldNode.Users {
		log.Debug().Msgf("%+v", user)
		if user.ID == t.UserID {
			return fmt.Errorf("user with id %s already exists", t.UserID)
		} else if user.Username == t.Username {
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

	// Assign a UUID to this specific installation of the app
	t.AppInstallation.ID = uuid.New().String()

	appState := &AppInstallationState{
		AppInstallation: t.AppInstallation,
		State:           AppLifecycleStateInstalling,
	}
	marshalledApp, err := json.Marshal(appState)
	if err != nil {
		return nil, err
	}

	for i, user := range oldNode.Users {
		if user.ID == t.UserID {
			return []byte(fmt.Sprintf(`[{
				"op": "add",
				"path": "/users/%d/app_installations/-",
				"value": %s
			}]`, i, string(marshalledApp))), nil
		}
	}
	return nil, fmt.Errorf("user with id %s not found", t.UserID)
}

func (t *StartInstallationTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	for _, user := range oldNode.Users {
		if user.ID == t.UserID {
			for _, app := range user.AppInstallations {
				if app.RegistryURLBase == t.RegistryURLBase && app.RegistryPackageID == t.RegistryPackageID {
					if app.Version == t.Version {
						return fmt.Errorf("app %s version %s for user %s found in state %s", t.Name, t.Version, t.UserID, app.State)
					} else {
						return fmt.Errorf("app %s for user %s found in state with different version %s", t.Name, t.UserID, app.Version)
					}
				}
			}
		}
	}

	return nil
}

type FinishInstallationTransition struct {
	UserID          string `json:"user_id"`
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

	for i, user := range oldNode.Users {
		if user.ID == t.UserID {
			for j, app := range user.AppInstallations {
				if app.RegistryURLBase == t.RegistryURLBase && app.RegistryPackageID == t.RegistryAppID {
					return []byte(fmt.Sprintf(`[{
						"op": "replace",
						"path": "/users/%d/app_installations/%d/state",
						"value": "%s"
					}]`, i, j, AppLifecycleStateInstalled)), nil
				}
			}
		}
	}
	return nil, fmt.Errorf("user with id %s not found", t.UserID)
}

func (t *FinishInstallationTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	for _, user := range oldNode.Users {
		if user.ID == t.UserID {
			for _, app := range user.AppInstallations {
				if app.RegistryURLBase == t.RegistryURLBase && app.RegistryPackageID == t.RegistryAppID {
					if app.State != "installing" {
						return fmt.Errorf("app %s for user %s is in state %s", app.Name, t.UserID, app.State)
					} else {
						return nil
					}
				}
			}
		}
	}

	return fmt.Errorf("app %s for user %s not found", t.RegistryAppID, t.UserID)
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
			"path": "/processes/-",
			"value": %s
		}]`, marshaled)), nil
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

	// Make sure that the user exists
	userExists := false
	appExists := false
	for _, user := range oldNode.Users {
		if user.ID == t.UserID {
			userExists = true

			// Make sure the user has the referenced application
			for _, app := range user.AppInstallations {
				if app.ID == t.AppID {
					appExists = true
				}
				if app.State != AppLifecycleStateInstalled {
					return fmt.Errorf("App with id %s for user %s is not in state %s", t.AppID, t.UserID, AppLifecycleStateInstalled)
				}
			}
		}
	}
	if !userExists {
		return fmt.Errorf("User with id %s does not exist", t.UserID)
	}
	if !appExists {
		return fmt.Errorf("App with id %s does not exist for user %s", t.AppID, t.UserID)
	}

	for _, proc := range oldNode.Processes {
		// Make sure that no process exists with the same ID
		if proc.ID == t.ID {
			return fmt.Errorf("Process with id %s already exists", t.ID)
		}
		// Make sure that no app with the same ID has a process
		if proc.AppID == t.AppID {
			return fmt.Errorf("App with id %s already has a process", t.AppID)
		}
	}

	return nil
}

type ProcessRunningTransition struct {
	ProcessID   string `json:"process_id"`
	ExtDriverID string `json:"ext_driver_id"`
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
	for i, proc := range oldNode.Processes {
		if proc.ID == t.ProcessID {
			proc.State = ProcessStateRunning
			proc.ExtDriverID = t.ExtDriverID

			marshaled, err := json.Marshal(proc)
			if err != nil {
				return nil, err
			}

			return []byte(fmt.Sprintf(`[{
				"op": "replace",
				"path": "/processes/%d",
				"value": %s
			}]`, i, marshaled)), nil
		}
	}
	return nil, fmt.Errorf("Process with id %s not found", t.ProcessID)
}

func (t *ProcessRunningTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	if t.ExtDriverID == "" {
		return fmt.Errorf("ext_driver_id cannot be empty")
	}

	// Make sure there is a matching process
	for _, proc := range oldNode.Processes {
		if proc.ID == t.ProcessID {
			if proc.State != ProcessStateStarting {
				return fmt.Errorf("Process with id %s is in state %s, must be in state %s", t.ProcessID, proc.State, ProcessStateStarting)
			}
			return nil
		}
	}

	return fmt.Errorf("Process with id %s not found", t.ProcessID)
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

	for i, proc := range oldNode.Processes {
		if proc.ID == t.ProcessID {
			return []byte(fmt.Sprintf(`[{
				"op": "remove",
				"path": "/processes/%d"
			}]`, i)), nil
		}
	}
	return nil, fmt.Errorf("Process with id %s not found", t.ProcessID)
}

func (t *ProcessStopTransition) Validate(oldState []byte) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState, &oldNode)
	if err != nil {
		return err
	}

	// Make sure there is a matching process
	for _, proc := range oldNode.Processes {
		if proc.ID == t.ProcessID {
			return nil
		}
	}

	return fmt.Errorf("Process with id %s not found", t.ProcessID)
}
