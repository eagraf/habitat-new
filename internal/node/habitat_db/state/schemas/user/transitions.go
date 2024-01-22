package user

import (
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
)

var TransitionInitialize = "initialize"

type InitalizationTransition struct {
	UserID    string `json:"user_id"`
	PublicKey string `json:"public_key"`
	Username  string `json:"username"`
}

func (t *InitalizationTransition) Type() string {
	return TransitionInitialize
}

func (t *InitalizationTransition) Patch(oldState *state.JSONState) ([]byte, error) {
	return []byte(fmt.Sprintf(`[{
		"op": "add",
		"path": "/user_id",
		"value": "%s"
	},
	{
		"op": "add",
		"path": "/username",
		"value": "%s"
	},
	{
		"op": "add",
		"path": "/public_key",
		"value": "%s"
	}]`, t.UserID, t.Username, t.PublicKey)), nil
}

func (t *InitalizationTransition) Validate(oldState *state.JSONState) error {
	return nil
}
