package controller

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/hdbms"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type NodeController interface {
	InitializeNodeDB() error
	AddUser(userID, username, certificate string) error
	GetUserByUsername(username string) (*node.User, error)
	InstallApp(userID string, newApp *node.AppInstallation) error
}

type BaseNodeController struct {
	databaseManager hdb.HDBManager
	nodeConfig      *config.NodeConfig
}

func (c *BaseNodeController) InitializeNodeDB() error {
	// Try initializing the database, is a noop if a database with the same name already exists
	_, err := c.databaseManager.CreateDatabase(constants.NodeDBDefaultName, node.SchemaName, generateInitState(c.nodeConfig))
	if err != nil {
		if _, ok := err.(*hdbms.DatabaseAlreadyExistsError); ok {
			log.Info().Msg("Node database already exists, doing nothing.")
		} else {
			return err
		}
	}

	return nil
}

func (c *BaseNodeController) InstallApp(userID string, newApp *node.AppInstallation) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
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

func (c *BaseNodeController) AddUser(userID, username, certificate string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
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

func (c *BaseNodeController) GetUserByUsername(username string) (*node.User, error) {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return nil, err
	}

	var nodeState node.NodeState
	err = json.Unmarshal(dbClient.Bytes(), &nodeState)
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

// TODO this is basically a placeholder until we actually have a way of generating
// the certificate for the node.
func generateInitState(nodeConfig *config.NodeConfig) []byte {
	nodeUUID := uuid.New().String()

	rootCert := nodeConfig.RootUserCertB64()

	return []byte(fmt.Sprintf(`{
			"node_id": "%s",
			"name": "My Habitat node",
			"certificate": "placeholder",
			"users": [
				{
					"id": "%d",
					"username": "%s",
					"certificate": "%s",
					"app_installations": []
				}
			]
		}`, nodeUUID, 0, constants.RootUsername, rootCert))
}
