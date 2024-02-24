package controller

import (
	"crypto/x509"
	"encoding/json"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	hdb_mocks "github.com/eagraf/habitat-new/internal/node/hdb/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestInitializeNodeDB(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig: &config.NodeConfig{
			RootUserCert: &x509.Certificate{
				Raw: []byte("root_cert"),
			},
		},
	}

	// Check that fakeInitState is based off of the config we pass in
	mockedManager.EXPECT().CreateDatabase(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		// signature of anonymous function must have the same number of input and output arguments as the mocked method.
		func(nodeName, schemaName string, initState []byte) (hdb.Client, error) {
			var state node.NodeState
			err := json.Unmarshal(initState, &state)
			assert.Nil(t, err)

			assert.Equal(t, 1, len(state.Users))
			assert.Equal(t, constants.RootUsername, state.Users[0].Username)
			assert.Equal(t, constants.RootUserID, state.Users[0].ID)
			assert.Equal(t, "cm9vdF9jZXJ0", state.Users[0].Certificate)

			return mockedClient, nil
		}).Times(1)
	err := controller.InitializeNodeDB()
	assert.Nil(t, err)

	mockedManager.EXPECT().CreateDatabase(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)
	err = controller.InitializeNodeDB()
	assert.NotNil(t, err)

	mockedManager.EXPECT().CreateDatabase(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &hdb.DatabaseAlreadyExistsError{}).Times(1)
	err = controller.InitializeNodeDB()
	assert.Nil(t, err)
}

func TestInstallAppController(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig:      &config.NodeConfig{},
	}

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockedClient, nil).Times(1)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.StartInstallationTransition{
				UserID: "0",
				AppInstallation: &node.AppInstallation{
					Name:    "app_name1",
					Version: "1",
					Package: node.Package{
						Driver:             "docker",
						RegistryURLBase:    "https://registry.com",
						RegistryPackageID:  "app_name1",
						RegistryPackageTag: "v1",
					},
				},
			},
		},
	)).Return(nil, nil).Times(1)

	err := controller.InstallApp("0", &node.AppInstallation{
		Name:    "app_name1",
		Version: "1",
		Package: node.Package{
			Driver:             "docker",
			RegistryURLBase:    "https://registry.com",
			RegistryPackageID:  "app_name1",
			RegistryPackageTag: "v1",
		},
	})
	assert.Nil(t, err)
}

var nodeState = &node.NodeState{
	Users: []*node.User{
		{
			ID:       "user_1",
			Username: "username_1",
			AppInstallations: []*node.AppInstallationState{
				{
					AppInstallation: &node.AppInstallation{
						ID: "app_1",
					},
					State: node.AppLifecycleStateInstalled,
				},
			},
		},
	},
}

func TestFinishAppInstallationController(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig:      &config.NodeConfig{},
	}

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockedClient, nil).Times(1)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.FinishInstallationTransition{
				UserID:          "user_1",
				RegistryURLBase: "https://registry.com",
				RegistryAppID:   "app_1",
			},
		},
	)).Return(nil, nil).Times(1)

	err := controller.FinishAppInstallation("user_1", "https://registry.com", "app_1")
	assert.Nil(t, err)
}

func TestStartProcessController(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig:      &config.NodeConfig{},
	}

	marshaledNodeState, err := json.Marshal(nodeState)
	if err != nil {
		t.Error(err)
	}

	mockedClient.EXPECT().Bytes().Return(marshaledNodeState).Times(1)

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockedClient, nil).Times(3)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.ProcessStartTransition{
				Process: &node.Process{
					ID:     "process_1",
					AppID:  "app_1",
					UserID: "user_1",
				},
				App: &node.AppInstallation{
					ID: "app_1",
				},
			},
		},
	)).Return(nil, nil).Times(1)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.ProcessRunningTransition{
				ProcessID: "process_1",
			},
		},
	)).Return(nil, nil).Times(1)

	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.ProcessStopTransition{
				ProcessID: "process_1",
			},
		},
	)).Return(nil, nil).Times(1)

	err = controller.StartProcess(&node.Process{
		ID:     "process_1",
		AppID:  "app_1",
		UserID: "user_1",
	})
	assert.Nil(t, err)

	// Test setting the process to running, and then stopping it.
	err = controller.SetProcessRunning("process_1")
	assert.Nil(t, err)

	err = controller.StopProcess("process_1")
	assert.Nil(t, err)
}

func TestAddUser(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig:      &config.NodeConfig{},
	}

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockedClient, nil).Times(1)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.AddUserTransition{
				UserID:      "user_1",
				Username:    "username_1",
				Certificate: "cert_1",
			},
		},
	)).Return(nil, nil).Times(1)

	err := controller.AddUser("user_1", "username_1", "cert_1")
	assert.Nil(t, err)
}

func TestGetUserByUsername(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	controller := &BaseNodeController{
		databaseManager: mockedManager,
		nodeConfig:      &config.NodeConfig{},
	}

	marshaledNodeState, err := json.Marshal(nodeState)
	if err != nil {
		t.Error(err)
	}

	mockedClient.EXPECT().Bytes().Return(marshaledNodeState).Times(2)

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockedClient, nil).Times(2)

	user, err := controller.GetUserByUsername("username_1")
	assert.Nil(t, err)
	assert.Equal(t, "user_1", user.ID)
	assert.Equal(t, "username_1", user.Username)

	// Test username not found
	user, err = controller.GetUserByUsername("username_2")
	assert.NotNil(t, err)
	assert.Nil(t, user)
}
