package node

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/rs/zerolog/log"
)

var (
	TransitionInitialize         = "initialize"
	TransitionAddUser            = "add_user"
	TransitionStartInstallation  = "start_installation"
	TransitionFinishInstallation = "finish_installation"
)

type InitalizationTransition struct {
	InitState *NodeState `json:"init_state"`
}

func (t *InitalizationTransition) Type() string {
	return TransitionInitialize
}

func (t *InitalizationTransition) Patch(oldState *state.JSONState) ([]byte, error) {
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

func (t *InitalizationTransition) Validate(oldState *state.JSONState) error {
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

func (t *AddUserTransition) Patch(oldState *state.JSONState) ([]byte, error) {
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

func (t *AddUserTransition) Validate(oldState *state.JSONState) error {

	var oldNode NodeState
	err := json.Unmarshal(oldState.Bytes(), &oldNode)
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
	UserID          string `json:"user_id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Driver          string `json:"driver"`
	RegistryURLBase string `json:"registry_url_base"`
	RegistryAppID   string `json:"registry_app_id"`
	RegistryTag     string `json:"registry_tag"`
}

func (t *StartInstallationTransition) Type() string {
	return TransitionStartInstallation
}

func (t *StartInstallationTransition) Patch(oldState *state.JSONState) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState.Bytes(), &oldNode)
	if err != nil {
		return nil, err
	}

	for i, user := range oldNode.Users {
		if user.ID == t.UserID {
			return []byte(fmt.Sprintf(`[{
				"op": "add",
				"path": "/users/%d/app_installations/-",
				"value": {
					"name": "%s",
					"version": "%s",
					"driver": "%s",
					"registry_url_base": "%s",
					"registry_app_id": "%s",
					"registry_tag": "%s",
					"state": "installing"
				}
			}]`, i, t.Name, t.Version, t.Driver, t.RegistryURLBase, t.RegistryAppID, t.RegistryTag)), nil
		}
	}
	return nil, fmt.Errorf("user with id %s not found", t.UserID)
}

func (t *StartInstallationTransition) Validate(oldState *state.JSONState) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState.Bytes(), &oldNode)
	if err != nil {
		return err
	}

	for _, user := range oldNode.Users {
		if user.ID == t.UserID {
			for _, app := range user.AppInstallations {
				if app.RegistryURLBase == t.RegistryURLBase && app.RegistryAppID == t.RegistryAppID {
					if app.Version == t.Version {
						return fmt.Errorf("app %s version %s for user %s found in state %s", t.Name, t.Version, t.UserID, app.Version)
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

func (t *FinishInstallationTransition) Patch(oldState *state.JSONState) ([]byte, error) {
	var oldNode NodeState
	err := json.Unmarshal(oldState.Bytes(), &oldNode)
	if err != nil {
		return nil, err
	}

	for i, user := range oldNode.Users {
		if user.ID == t.UserID {
			for j, app := range user.AppInstallations {
				if app.RegistryURLBase == t.RegistryURLBase && app.RegistryAppID == t.RegistryAppID {
					return []byte(fmt.Sprintf(`[{
						"op": "replace",
						"path": "/users/%d/app_installations/%d/state",
						"value": "installed"
					}]`, i, j)), nil
				}
			}
		}
	}
	return nil, fmt.Errorf("user with id %s not found", t.UserID)
}

func (t *FinishInstallationTransition) Validate(oldState *state.JSONState) error {
	var oldNode NodeState
	err := json.Unmarshal(oldState.Bytes(), &oldNode)
	if err != nil {
		return err
	}

	for _, user := range oldNode.Users {
		if user.ID == t.UserID {
			for _, app := range user.AppInstallations {
				if app.RegistryURLBase == t.RegistryURLBase && app.RegistryAppID == t.RegistryAppID {
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
