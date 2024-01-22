package node

import (
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
)

var TransitionInitialize = "initialize"

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
