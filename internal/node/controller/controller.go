package controller

import (
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const NodeDBDefaultName = "node"

type NodeController struct {
	databaseManager *habitat_db.DatabaseManager
}

func (c *NodeController) InitializeNodeDB() error {
	// Try initializing the database, is a noop if a database with the same name already exists
	_, err := c.databaseManager.CreateDatabase(NodeDBDefaultName, node.SchemaName, generateInitState())
	if err != nil {
		if _, ok := err.(*habitat_db.DatabaseAlreadyExistsError); ok {
			log.Info().Msg("Node database already exists, doing nothing.")
		} else {
			return err
		}
	}

	return nil
}

func (c *NodeController) AddUser(userID, username, publicKey string) error {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = db.Controller.ProposeTransitions([]state.Transition{
		&node.AddUserTransition{
			UserID:    userID,
			Username:  username,
			PublicKey: publicKey,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// TODO this is basically a placeholder until we actually have a way of generating
// the certificate for the node.
func generateInitState() []byte {
	uuid := uuid.New().String()
	return []byte(fmt.Sprintf(`{
			"node_id": "%s",
			"name": "My Habitat node",
			"certificate": "placeholder"
		}`, uuid))
}
