package controller

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
)

func (c *BaseNodeController) GetNodeState() (*node.State, error) {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return nil, err
	}

	stateBytes := dbClient.Bytes()
	if err != nil {
		return nil, err
	}

	var nodeState node.State
	err = json.Unmarshal(stateBytes, &nodeState)
	if err != nil {
		return nil, err
	}

	return &nodeState, nil
}
