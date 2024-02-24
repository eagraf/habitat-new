package processes

import (
	"errors"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/processes/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestProcessRestorer(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockDriver := mocks.NewMockProcessDriver(ctrl)

	pm := newBaseProcessManager()
	pm.processDrivers["test"] = mockDriver

	pr := &ProcessRestorer{
		processManager: pm,
	}

	restoreUpdate, err := stateUpdateTestHelper(&node.InitalizationTransition{}, &node.NodeState{
		Users: []*node.User{
			{
				ID: "user1",
				AppInstallations: []*node.AppInstallationState{
					{
						AppInstallation: &node.AppInstallation{
							ID:     "app1",
							Name:   "appname1",
							Driver: "test",
						},
					},
					{
						AppInstallation: &node.AppInstallation{
							ID:     "app3",
							Name:   "appname3",
							Driver: "test",
						},
					},
				},
			},
		},
		Processes: []*node.ProcessState{
			{
				Process: &node.Process{
					ID:    "proc1",
					AppID: "app1",
				},
				State: node.ProcessStateRunning,
			},
			// This process was not in a running state, should not be started
			{
				Process: &node.Process{
					ID:    "proc2",
					AppID: "app1",
				},
				State: node.ProcessStateStarting,
			},
			// The app for this process does not exist, it should not be started
			{
				Process: &node.Process{
					ID:    "proc3",
					AppID: "app2",
				},
				State: node.ProcessStateRunning,
			},
			{
				Process: &node.Process{
					ID:    "proc4",
					AppID: "app3",
				},
				State: node.ProcessStateRunning,
			},
		},
	})
	assert.Nil(t, err)

	mockDriver.EXPECT().StartProcess(
		gomock.Eq(
			&node.Process{
				ID:    "proc1",
				AppID: "app1",
			},
		),
		gomock.Eq(
			&node.AppInstallation{
				ID:     "app1",
				Name:   "appname1",
				Driver: "test",
			},
		),
	).Return("ext_proc1", nil).Times(1)

	mockDriver.EXPECT().StartProcess(
		gomock.Eq(
			&node.Process{
				ID:    "proc4",
				AppID: "app3",
			},
		),
		gomock.Eq(
			&node.AppInstallation{
				ID:     "app3",
				Name:   "appname3",
				Driver: "test",
			},
		),
	).Return("", errors.New("Error starting process")).Times(1)

	err = pr.Restore(restoreUpdate)
	assert.Nil(t, err)

}
