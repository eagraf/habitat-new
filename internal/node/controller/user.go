package controller

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
)

func (c *NodeController) AddUser(userID, username, certificate string) error {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = db.Controller.ProposeTransitions([]state.Transition{
		&node.AddUserTransition{
			UserID:      userID,
			Username:    username,
			Certificate: certificate,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *NodeController) GetUserByUsername(username string) (*node.User, error) {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return nil, err
	}

	var nodeState node.NodeState
	err = json.Unmarshal(db.Controller.Bytes(), &nodeState)
	if err != nil {
		return nil, err
	}

	for _, user := range nodeState.Users {
		if user.Username == username {
			return user, err
		}
	}

	return nil, fmt.Errorf("user with username %s not found", username)
}
