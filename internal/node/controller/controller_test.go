package controller

import (
	"context"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	hdb_mocks "github.com/eagraf/habitat-new/internal/node/hdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func fakeInitState(rootUserCert, rootUserID, rootUsername string) *node.State {
	initState, err := node.GetEmptyStateForVersion(node.LatestVersion)
	if err != nil {
		panic(err)
	}

	initState.Users[rootUserID] = &node.User{
		ID:       rootUserID,
		Username: rootUsername,
	}

	return initState
}

func setupNodeDBTest(ctrl *gomock.Controller, t *testing.T) (*hdb_mocks.MockHDBManager, *hdb_mocks.MockClient) {

	mockedManager := hdb_mocks.NewMockHDBManager(ctrl)
	mockedClient := hdb_mocks.NewMockClient(ctrl)

	initState := fakeInitState("fake_cert", "fake_user_id", "fake_username")
	transitions := []hdb.Transition{
		&node.InitalizationTransition{
			InitState: initState,
		},
	}

	// Check that fakeInitState is based off of the config we pass in
	mockedManager.EXPECT().CreateDatabase(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		// signature of anonymous function must have the same number of input and output arguments as the mocked method.
		func(ctx context.Context, nodeName, schemaName string, initTransitions []hdb.Transition) (hdb.Client, error) {
			require.Equal(t, 1, len(initTransitions))

			initStateTransition := initTransitions[0]
			require.Equal(t, hdb.TransitionInitialize, initStateTransition.Type())

			state := initStateTransition.(*node.InitalizationTransition).InitState

			assert.Equal(t, 1, len(state.Users))
			assert.Equal(t, "fake_username", state.Users["fake_user_id"].Username)
			assert.Equal(t, "fake_user_id", state.Users["fake_user_id"].ID)

			return mockedClient, nil
		}).Times(1)

	err := InitializeNodeDB(context.Background(), mockedManager, transitions)
	require.NoError(t, err)

	return mockedManager, mockedClient
}

func TestInitializeNodeDB(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager, _ := setupNodeDBTest(ctrl, t)

	mockedManager.EXPECT().CreateDatabase(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)
	err := InitializeNodeDB(context.Background(), mockedManager, nil)
	require.NotNil(t, err)

	mockedManager.EXPECT().CreateDatabase(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &hdb.DatabaseAlreadyExistsError{}).Times(1)
	err = InitializeNodeDB(context.Background(), mockedManager, nil)
	require.Nil(t, err)
}

func TestMigrationController(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedManager, mockClient := setupNodeDBTest(ctrl, t)

	nodeState := &node.State{
		SchemaVersion: "v0.0.2",
	}
	marshaled, err := nodeState.Bytes()
	assert.Equal(t, nil, err)

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockClient, nil).Times(1)
	mockClient.EXPECT().Bytes().Return(marshaled).Times(1)
	mockClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.MigrationTransition{
				TargetVersion: "v0.0.3",
			},
		},
	)).Return(nil, nil).Times(1)

	err = MigrateNodeDB(mockedManager, "v0.0.3")
	assert.Nil(t, err)

	// Test no-op by migrating to the same version
	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockClient, nil).Times(1)
	mockClient.EXPECT().Bytes().Return(marshaled).Times(1)
	mockClient.EXPECT().ProposeTransitions(gomock.Any()).Times(0)

	err = MigrateNodeDB(mockedManager, "v0.0.2")
	assert.Nil(t, err)

	// Test  migrating to a lower version

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockClient, nil).Times(1)
	mockClient.EXPECT().Bytes().Return(marshaled).Times(1)
	mockClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.MigrationTransition{
				TargetVersion: "v0.0.1",
			},
		},
	)).Times(1)

	err = MigrateNodeDB(mockedManager, "v0.0.1")
	assert.Nil(t, err)
}

var nodeState = &node.State{
	Users: map[string]*node.User{
		"user_1": {
			ID:       "user_1",
			Username: "username_1",
		},
	},
	AppInstallations: map[string]*node.AppInstallation{
		"app_1": {
			ID:     "app_1",
			UserID: "0",
		},
	},
}

/*
func TestAddUser(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockedPDSClient, mockedManager, mockedClient := setupNodeDBTest(ctrl, t)

	// Test successful add user

	mockedPDSClient.EXPECT().CreateAccount("user@user.com", "username_1", "password").Return(map[string]interface{}{
		"did": "did_1",
	}, nil).Times(1)

	mockedManager.EXPECT().GetDatabaseClientByName(constants.NodeDBDefaultName).Return(mockedClient, nil).Times(1)
	mockedClient.EXPECT().ProposeTransitions(gomock.Eq(
		[]hdb.Transition{
			&node.AddUserTransition{
				Username:    "username_1",
				Certificate: "cert_1",
				AtprotoDID:  "did_1",
			},
		},
	)).Return(nil, nil).Times(1)

	_, err := AddUser("user_1", "user@user.com", "username_1", "password", "cert_1")
	assert.Nil(t, err)

	// Test error from empty did.
	mockedPDSClient.EXPECT().CreateAccount("user@user.com", "username_1", "password").Return(map[string]interface{}{
		"did": "",
	}, nil).Times(1)

	_, err = controller.AddUser("user_1", "user@user.com", "username_1", "password", "cert_1")
	assert.NotNil(t, err)

	mockedPDSClient.EXPECT().CreateAccount("user@user.com", "username_1", "password").Return(map[string]interface{}{}, nil).Times(1)

	_, err = controller.AddUser("user_1", "user@user.com", "username_1", "password", "cert_1")
	assert.NotNil(t, err)

	// Test create account error
	mockedPDSClient.EXPECT().CreateAccount("user@user.com", "username_1", "password").Return(nil, errors.New("failed to create account")).Times(1)

	_, err = controller.AddUser("user_1", "user@user.com", "username_1", "password", "cert_1")
	assert.NotNil(t, err)

}

*/
