package controller

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// NodeController is an interface to manage common admin actions on a Habitat node.
// For example, installing apps or adding users. This will likely expand to be a much bigger API as we move forward.

type NodeController interface {
	InitializeNodeDB() error
	AddUser(userID, username, certificate string) error
	GetUserByUsername(username string) (*node.User, error)
	InstallApp(userID string, newApp *node.AppInstallation) error
	FinishAppInstallation(userID string, appID, registryURLBase, registryPackageID string) error
	StartProcess(process *node.Process) error
	SetProcessRunning(processID string) error
	StopProcess(processID string) error
}

type BaseNodeController struct {
	databaseManager hdb.HDBManager
	nodeConfig      *config.NodeConfig
}

// InitializeNodeDB tries initializing the database; it is a noop if a database with the same name already exists
func (c *BaseNodeController) InitializeNodeDB() error {
	_, err := c.databaseManager.CreateDatabase(constants.NodeDBDefaultName, node.SchemaName, generateInitState(c.nodeConfig))
	if err != nil {
		if _, ok := err.(*hdb.DatabaseAlreadyExistsError); ok {
			log.Info().Msg("Node database already exists, doing nothing.")
		} else {
			return err
		}
	}

	return nil
}

// InstallApp attempts to install the given app installation, with the userID as the action initiato.
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
	return err
}

// FinishAppInstallation marks the app lifecycle state as installed
func (c *BaseNodeController) FinishAppInstallation(userID string, appID, registryURLBase, registryAppID string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.FinishInstallationTransition{
			UserID:          userID,
			AppID:           appID,
			RegistryURLBase: registryURLBase,
			RegistryAppID:   registryAppID,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseNodeController) StartProcess(process *node.Process) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	var nodeState node.NodeState
	err = json.Unmarshal(dbClient.Bytes(), &nodeState)
	if err != nil {
		return nil
	}

	var user *node.User
	for _, u := range nodeState.Users {
		if u.ID == process.UserID {
			user = u
		}
	}
	if user == nil {
		return fmt.Errorf("user %s not found", process.UserID)
	}

	var app *node.AppInstallation
	appState, ok := nodeState.AppInstallations[process.AppID]
	if !ok {
		return fmt.Errorf("app with ID %s not found", process.AppID)
	}
	app = appState.AppInstallation

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.ProcessStartTransition{
			Process: process,
			App:     app,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseNodeController) SetProcessRunning(processID string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.ProcessRunningTransition{
			ProcessID: processID,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseNodeController) StopProcess(processID string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.ProcessStopTransition{
			ProcessID: processID,
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
	return err
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
			"users": {
				"%s": {
					"id": "%s",
					"username": "%s",
					"certificate": "%s",
					"app_installations": []
				}
			}
		}`, nodeUUID, constants.RootUserID, constants.RootUserID, constants.RootUsername, rootCert))
}
