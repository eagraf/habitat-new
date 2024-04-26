package processes

import (
	"testing"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/processes/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestProcessManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDriver := mocks.NewMockProcessDriver(ctrl)

	mockDriver.EXPECT().Type().Return("test")
	pm := NewProcessManager([]ProcessDriver{mockDriver})

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
	err := pm.StartProcess(
		&node.Process{
			ID:     "proc1",
			Driver: "test",
		},
		&node.AppInstallation{
			ID:   "app1",
			Name: "appname1",
			Package: node.Package{
				Driver: "test",
			},
		},
	)
	assert.Nil(t, err)

	procs, err := pm.ListProcesses()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(procs))

	proc, err := pm.GetProcess("proc1")
	assert.Nil(t, err)
	assert.Equal(t, "proc1", proc.ID)

	proc, err = pm.GetProcess("proc2")
	assert.NotNil(t, err)
	assert.Nil(t, proc)

	mockDriver.EXPECT().StopProcess("ext_proc1").Return(nil)

	err = pm.StopProcess("proc1")
	assert.Nil(t, err)

	err = pm.StopProcess("proc2")
	assert.NotNil(t, err)

	procs, err = pm.ListProcesses()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(procs))

}
