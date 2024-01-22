package controller

import (
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
	"github.com/google/uuid"
)

const NodeDBDefaultName = "node"

type NodeController struct {
	databaseManager *habitat_db.DatabaseManager
}

func (c *NodeController) InitializeNodeDB() error {
	_, err := c.databaseManager.GetDatabaseByName("node")
	if err != nil {
		if _, ok := err.(*habitat_db.DatabaseNotFoundError); ok {
			// Database not found, let's initialize it.
			_, err := c.databaseManager.CreateDatabase(NodeDBDefaultName, node.SchemaName, generateInitState())
			if err != nil {
				return err
			}
		} else {
			return nil
		}
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
