package processes

import (
	"encoding/json"
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	ctrl_mocks "github.com/eagraf/habitat-new/internal/node/controller/mocks"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/processes/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func stateUpdateTestHelper(transition hdb.Transition, newState *node.NodeState) (*hdb.StateUpdate, error) {
	transBytes, err := json.Marshal(transition)
	if err != nil {
		return nil, err
	}

	stateBytes, err := json.Marshal(newState)
	if err != nil {
		return nil, err
	}

	return &hdb.StateUpdate{
		SchemaType:     node.SchemaName,
		Transition:     transBytes,
		TransitionType: transition.Type(),
		NewState:       stateBytes,
	}, nil
}

func TestSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDriver := mocks.NewMockProcessDriver(ctrl)

	pm := newBaseProcessManager()
	pm.processDrivers["test"] = mockDriver

	nc := ctrl_mocks.NewMockNodeController(ctrl)

	startProcessExecutor := &StartProcessExecutor{
		processManager: pm,
		nodeController: nc,
	}

	startProcessStateUpdate, err := stateUpdateTestHelper(&node.ProcessStartTransition{
		Process: &node.Process{
			ID:     "proc1",
			Driver: "test",
		},
		App: &node.AppInstallation{
			ID:     "app1",
			Name:   "appname1",
			Driver: "test",
		},
	}, &node.NodeState{
		Processes: []*node.ProcessState{},
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
				ID:     "app1",
				Name:   "appname1",
				Driver: "test",
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
