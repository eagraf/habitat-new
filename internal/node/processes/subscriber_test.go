package processes

import (
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/core/state/node/test_helpers"
	ctrl_mocks "github.com/eagraf/habitat-new/internal/node/controller/mocks"
	"github.com/eagraf/habitat-new/internal/node/processes/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDriver := mocks.NewMockProcessDriver(ctrl)

	mockDriver.EXPECT().Type().Return("test")
	pm := NewProcessManager([]ProcessDriver{mockDriver})

	nc := ctrl_mocks.NewMockNodeController(ctrl)

	startProcessExecutor := &StartProcessExecutor{
		processManager: pm,
		nodeController: nc,
	}

	startProcessStateUpdate, err := test_helpers.StateUpdateTestHelper(&node.ProcessStartTransition{
		Process: &node.Process{
			ID:     "proc1",
			Driver: "test",
		},
		App: &node.AppInstallation{
			ID:   "app1",
			Name: "appname1",
			Package: node.Package{
				Driver: "test",
			},
		},
	}, &node.NodeState{
		Processes: map[string]*node.ProcessState{},
	})
	assert.Nil(t, err)

	shouldExecute, err := startProcessExecutor.ShouldExecute(startProcessStateUpdate)
	assert.Nil(t, err)
	assert.Equal(t, true, shouldExecute)

	mockDriver.EXPECT().StartProcess(
		gomock.Eq(
			&node.Process{
				ID:     "proc1",
				Driver: "test",
			},
		),
		gomock.Eq(
			&node.AppInstallation{
				ID:   "app1",
				Name: "appname1",
				Package: node.Package{
					Driver: "test",
				},
			},
		),
	).Return("ext_proc1", nil)

	nc.EXPECT().SetProcessRunning("proc1").Return(nil)

	err = startProcessExecutor.Execute(startProcessStateUpdate)
	assert.Nil(t, err)

	shouldExecute, err = startProcessExecutor.ShouldExecute(startProcessStateUpdate)
	assert.Nil(t, err)
	assert.Equal(t, false, shouldExecute)
}
