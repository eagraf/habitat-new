package controller

import (
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
)

func (c *NodeController) InstallApp(userID string, newApp *node.AppInstallation) error {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = db.Controller.ProposeTransitions([]state.Transition{
		&node.StartInstallationTransition{
			UserID:          userID,
			AppInstallation: newApp,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
