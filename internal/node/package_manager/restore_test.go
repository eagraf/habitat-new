package package_manager

import (
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/core/state/node/test_helpers"
	"github.com/eagraf/habitat-new/internal/node/package_manager/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRestore(t *testing.T) {
	restoreUpdate, err := test_helpers.StateUpdateTestHelper(&node.InitalizationTransition{}, &node.NodeState{
		Users: []*node.User{
			{
				ID: "user1",
				AppInstallations: []*node.AppInstallationState{
					{
						AppInstallation: &node.AppInstallation{
							ID: "app1",
						},
						State: node.AppLifecycleStateInstalling,
					},
					{
						AppInstallation: &node.AppInstallation{
							ID: "app2",
						},
						State: node.AppLifecycleStateInstalled,
					},
				},
			},
		},
	})
	assert.Nil(t, err)

	ctrl := gomock.NewController(t)

	pm := mocks.NewMockPackageManager(ctrl)

	pmRestorer := &PackageManagerRestorer{
		packageManager: pm,
	}

	pm.EXPECT().InstallPackage(gomock.Any(), gomock.Any()).Times(1)

	err = pmRestorer.Restore(restoreUpdate)
	assert.Nil(t, err)

}
