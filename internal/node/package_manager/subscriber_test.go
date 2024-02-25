package package_manager

import (
	"errors"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/core/state/node/test_helpers"
	controller_mocks "github.com/eagraf/habitat-new/internal/node/controller/mocks"
	"github.com/eagraf/habitat-new/internal/node/package_manager/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSubscriber(t *testing.T) {
	stateUpdate, err := test_helpers.StateUpdateTestHelper(
		&node.StartInstallationTransition{
			UserID: "user1",
			AppInstallation: &node.AppInstallation{
				ID:      "app1",
				Name:    "appname1",
				Version: "v1",
				Package: node.Package{
					Driver:            "test",
					RegistryURLBase:   "registry.com",
					RegistryPackageID: "package1",
				},
			},
		},
		&node.NodeState{
			Users: []*node.User{
				{
					ID:               "user1",
					AppInstallations: []*node.AppInstallationState{},
				},
			},
		})
	assert.Nil(t, err)

	ctrl := gomock.NewController(t)

	pm := mocks.NewMockPackageManager(ctrl)
	nc := controller_mocks.NewMockNodeController(ctrl)

	installAppExecutor := &InstallAppExecutor{
		packageManager: pm,
		nodeController: nc,
	}

	// Test not installed
	pm.EXPECT().IsInstalled(gomock.Eq(&node.Package{
		Driver:            "test",
		RegistryURLBase:   "registry.com",
		RegistryPackageID: "package1",
	}), gomock.Eq("v1")).Return(false, nil).Times(1)

	should, err := installAppExecutor.ShouldExecute(stateUpdate)
	assert.Nil(t, err)
	assert.Equal(t, true, should)

	// Test that ShouldExecute returns false when it is installed
	pm.EXPECT().IsInstalled(gomock.Eq(&node.Package{
		Driver:            "test",
		RegistryURLBase:   "registry.com",
		RegistryPackageID: "package1",
	}), gomock.Eq("v1")).Return(true, nil).Times(1)

	should, err = installAppExecutor.ShouldExecute(stateUpdate)
	assert.Nil(t, err)
	assert.Equal(t, false, should)

	// Test installing the package

	pm.EXPECT().InstallPackage(gomock.Eq(&node.Package{
		Driver:            "test",
		RegistryURLBase:   "registry.com",
		RegistryPackageID: "package1",
	}), gomock.Eq("v1")).Return(nil).Times(1)

	err = installAppExecutor.Execute(stateUpdate)
	assert.Nil(t, err)

	nc.EXPECT().FinishAppInstallation(gomock.Eq("user1"), gomock.Eq("registry.com"), gomock.Eq("package1")).Return(nil).Times(1)

	err = installAppExecutor.PostHook(stateUpdate)
	assert.Nil(t, err)

	// Test installation failure from driver

	pm.EXPECT().InstallPackage(gomock.Eq(&node.Package{
		Driver:            "test",
		RegistryURLBase:   "registry.com",
		RegistryPackageID: "package1",
	}), gomock.Eq("v1")).Return(errors.New("Couldn't install")).Times(1)

	err = installAppExecutor.Execute(stateUpdate)
	assert.NotNil(t, err)

	assert.Equal(t, node.TransitionStartInstallation, installAppExecutor.TransitionType())
}
