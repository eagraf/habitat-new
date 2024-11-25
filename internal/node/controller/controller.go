package controller

import (
	"fmt"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/semver"
)

// NodeController is an interface to manage common admin actions on a Habitat node.
// For example, installing apps or adding users. This will likely expand to be a much bigger API as we move forward.

type NodeController interface {
	InitializeNodeDB() error
	MigrateNodeDB(targetVersion string) error

	AddUser(userID, email, handle, password, certificate string) (types.PDSCreateAccountResponse, error)
	GetUserByUsername(username string) (*node.User, error)

	InstallApp(userID string, newApp *node.AppInstallation, newProxyRules []*node.ReverseProxyRule) error
	FinishAppInstallation(userID string, appID, registryURLBase, registryPackageID string, startAfterInstall bool) error
	UpgradeApp(appID string, newAppInstallation *node.AppInstallation, newProxyRules []*node.ReverseProxyRule, version string) error
	FinishAppUpgrade(appID string, startAfterUpgrade bool) error
	GetAppByID(appID string) (*node.AppInstallation, error)

	StartProcess(appID string) error
	SetProcessRunning(processID string, extProcessID string) error
	StopProcess(processID string) error
	FinishProcessStop(processID string) error

	GetNodeState() (*node.State, error)
}

type BaseNodeController struct {
	databaseManager     hdb.HDBManager
	nodeConfig          *config.NodeConfig
	pdsClient           PDSClientI
	stateUpdatesChannel pubsub.Channel[hdb.StateUpdate]
}

func NewNodeController(habitatDBManager hdb.HDBManager, config *config.NodeConfig, stateUpdatesChannel pubsub.Channel[hdb.StateUpdate]) (*BaseNodeController, error) {
	controller := &BaseNodeController{
		databaseManager:     habitatDBManager,
		nodeConfig:          config,
		pdsClient:           NewPDSClient(config),
		stateUpdatesChannel: stateUpdatesChannel,
	}
	return controller, nil
}

// InitializeNodeDB tries initializing the database; it is a noop if a database with the same name already exists
func (c *BaseNodeController) InitializeNodeDB() error {

	initialTransitions, err := initTranstitions(c.nodeConfig)
	if err != nil {
		return err
	}

	_, err = c.databaseManager.CreateDatabase(constants.NodeDBDefaultName, node.SchemaName, initialTransitions)
	if err != nil {
		if _, ok := err.(*hdb.DatabaseAlreadyExistsError); ok {
			log.Info().Msg("Node database already exists, doing nothing.")
		} else {
			log.Error().Msgf("Error creating node database: %s", err)
			return err
		}
	}

	return nil
}

func (c *BaseNodeController) MigrateNodeDB(targetVersion string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	nodeState, err := c.GetNodeState()
	if err != nil {
		return err
	}

	// No-op if version is already the target
	if semver.Compare(nodeState.SchemaVersion, targetVersion) == 0 {
		return nil
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.MigrationTransition{
			TargetVersion: targetVersion,
		},
	})
	return err
}

// InstallApp attempts to install the given app installation, with the userID as the action initiato.
func (c *BaseNodeController) InstallApp(userID string, newApp *node.AppInstallation, newProxyRules []*node.ReverseProxyRule) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.StartInstallationTransition{
			UserID:                 userID,
			AppInstallation:        newApp,
			NewProxyRules:          newProxyRules,
			StartAfterInstallation: true,
		},
	})
	return err
}

// FinishAppInstallation marks the app lifecycle state as installed
func (c *BaseNodeController) FinishAppInstallation(userID string, appID, registryURLBase, registryAppID string, startAfterInstall bool) error {
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

			StartAfterInstallation: startAfterInstall,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseNodeController) UpgradeApp(appID string, newAppInstallation *node.AppInstallation, newProxyRules []*node.ReverseProxyRule, version string) error {
	nodeState, err := c.GetNodeState()
	if err != nil {
		return err
	}

	// Get process associated with app if it's running
	previouslyRunning := true
	process, err := nodeState.GetProcessForApp(appID)
	if err != nil {
		if _, ok := err.(node.ErrNotFound); ok {
			// App is not running, so we can just upgrade it
			previouslyRunning = false
		} else {
			return err
		}
	}

	if previouslyRunning {

		err = c.StopProcess(process.ID)
		if err != nil {
			return err
		}
		err := c.WaitForState(func(updatedState hdb.State) (bool, error) {
			state, ok := updatedState.(*node.State)
			if !ok {
				return false, fmt.Errorf("state not of type node.State")
			}
			if _, ok := state.Processes[process.ID]; !ok {
				return false, fmt.Errorf("process %s not found", process.ID)
			}

			return state.Processes[process.ID].State == node.ProcessStateStopped, nil
		})
		if err != nil {
			return err
		}
	}

	// Now we can upgrade the app
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.StartAppUpgradeTransition{
			AppID:              appID,
			NewAppInstallation: newAppInstallation,
			NewProxyRules:      newProxyRules,
			StartAfterUpgrade:  !previouslyRunning,
		},
	})
	if err != nil {
		return err
	}

	// Wait for the app to transition back to the upgrading state
	err = c.WaitForState(func(updatedState hdb.State) (bool, error) {
		state, ok := updatedState.(*node.State)
		if !ok {
			return false, fmt.Errorf("state update is not a node state")
		}
		if _, ok := state.AppInstallations[appID]; !ok {
			return false, fmt.Errorf("app %s not found", appID)
		}
		return state.AppInstallations[appID].State == node.AppLifecycleStateUpgrading, nil
	})
	if err != nil {
		return err
	}

	// Now, wait for the app to transition back to the installed state
	// We waited for both in order to avoid race conditions
	err = c.WaitForState(func(updatedState hdb.State) (bool, error) {
		state, ok := updatedState.(*node.State)
		if !ok {
			return false, fmt.Errorf("state update is not a node state")
		}
		if _, ok := state.AppInstallations[appID]; !ok {
			return false, fmt.Errorf("app %s not found", appID)
		}
		return state.AppInstallations[appID].State == node.AppLifecycleStateInstalled, nil
	})
	if err != nil {
		return err
	}

	// If we got here, the app is installed and we can start the process if needed
	if previouslyRunning {
		return c.StartProcess(appID)
	}

	return nil
}

func (c *BaseNodeController) FinishAppUpgrade(appID string, startAfterUpgrade bool) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.FinishAppUpgradeTransition{
			AppID:             appID,
			StartAfterUpgrade: startAfterUpgrade,
		},
	})

	return err
}

func (c *BaseNodeController) GetAppByID(appID string) (*node.AppInstallation, error) {
	nodeState, err := c.GetNodeState()
	if err != nil {
		return nil, err
	}

	app, ok := nodeState.AppInstallations[appID]
	if !ok {
		return nil, fmt.Errorf("app with ID %s not found", appID)
	}

	return app.AppInstallation, nil
}

func (c *BaseNodeController) StartProcess(appID string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.ProcessStartTransition{
			AppID: appID,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseNodeController) SetProcessRunning(processID string, extProcessID string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.ProcessRunningTransition{
			ProcessID:    processID,
			ExtProcessID: extProcessID,
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

func (c *BaseNodeController) FinishProcessStop(processID string) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.FinishProcessStopTransition{
			ProcessID: processID,
		},
	})
	return err
}

func (c *BaseNodeController) AddUser(userID, email, handle, password, certificate string) (types.PDSCreateAccountResponse, error) {

	inviteCode, err := c.pdsClient.GetInviteCode(c.nodeConfig)
	if err != nil {
		return nil, err
	}

	resp, err := c.pdsClient.CreateAccount(c.nodeConfig, email, handle, password, inviteCode)
	if err != nil {
		return nil, err
	}
	userDID := ""
	did, ok := resp["did"]
	if ok {
		if did == nil {
			return nil, fmt.Errorf("PDS response did not contain a DID (nil)")
		}

		userDID = did.(string)
	}

	if !ok || userDID == "" {
		return nil, fmt.Errorf("PDS response did not contain a DID")
	}

	dbClient, err := c.databaseManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		return nil, err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.AddUserTransition{
			Username:    handle,
			Certificate: certificate,
			AtprotoDID:  userDID,
		},
	})
	return resp, err
}

func (c *BaseNodeController) GetUserByUsername(username string) (*node.User, error) {
	nodeState, err := c.GetNodeState()
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
