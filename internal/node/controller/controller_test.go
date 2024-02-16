package controller

import (
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	hdb_mocks "github.com/eagraf/habitat-new/internal/node/hdb/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

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
					Name:            "app_name1",
					Version:         "1",
					Driver:          "docker",
					RegistryURLBase: "https://registry.com",
					RegistryAppID:   "app_name1",
					RegistryTag:     "v1",
				},
			},
		},
	)).Return(nil, nil).Times(1)

	err := controller.InstallApp("0", &node.AppInstallation{
		Name:            "app_name1",
		Version:         "1",
		Driver:          "docker",
		RegistryURLBase: "https://registry.com",
		RegistryAppID:   "app_name1",
		RegistryTag:     "v1",
	})
	assert.Nil(t, err)
}
