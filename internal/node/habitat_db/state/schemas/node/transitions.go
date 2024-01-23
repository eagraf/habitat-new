package node

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/rs/zerolog/log"
)

var (
	TransitionInitialize = "initialize"
	TransitionAddUser    = "add_user"
)

type InitalizationTransition struct {
	NodeID      string `json:"node_id"`
	Certificate string `json:"certificate"`
	Name        string `json:"name"`
}

func (t *InitalizationTransition) Type() string {
	return TransitionInitialize
}

func (t *InitalizationTransition) Patch(oldState *state.JSONState) ([]byte, error) {
	return []byte(fmt.Sprintf(`[{
		"op": "add",
		"path": "/node_id",
		"value": "%s"
	},
	{
		"op": "add",
		"path": "/name",
		"value": "%s"
	},
	{
		"op": "add",
		"path": "/certificate",
		"value": "%s"
	}]`, t.NodeID, t.Name, t.Certificate)), nil
}

func (t *InitalizationTransition) Validate(oldState *state.JSONState) error {
	return nil
}

type AddUserTransition struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
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
			"public_key": "%s"
		}
	}]`, t.UserID, t.Username, t.PublicKey)), nil
}

func (t *AddUserTransition) Validate(oldState *state.JSONState) error {

	var oldNode NodeState
	err := json.Unmarshal(oldState.Bytes(), &oldNode)
	if err != nil {
		return err
	}

	log.Debug().Msgf("%+v", oldNode)
	log.Debug().Msgf("%+v", t)
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
