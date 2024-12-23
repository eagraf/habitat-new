package process

/*
func TestSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDriver := mocks.NewMockProcessDriver(ctrl)

	mockDriver.EXPECT().Type().Return("test")
	pm := NewProcessManager([]Driver{mockDriver})

	nc := ctrl_mocks.NewMockNodeController(ctrl)

	startProcessExecutor := &StartProcessExecutor{
		processManager: pm,
		nodeController: nc,
	}

	startProcessStateUpdate, err := test_helpers.StateUpdateTestHelper(&node.ProcessStartTransition{
		AppID: "app1",
	}, &node.State{
		AppInstallations: map[string]*node.AppInstallationState{
			"app1": {
				AppInstallation: &node.AppInstallation{
					UserID: "0",
					ID:     "app1",
					Package: node.Package{
						Driver: "test",
					},
				},
			},
		},
		Processes: map[string]*node.ProcessState{},
	})
	assert.Nil(t, err)

	shouldExecute, err := startProcessExecutor.ShouldExecute(startProcessStateUpdate)
	assert.Nil(t, err)
	assert.Equal(t, true, shouldExecute)

	mockDriver.EXPECT().StartProcess(gomock.Any(), gomock.Any()).DoAndReturn(
		func(process *node.Process, app *node.AppInstallation) (string, error) {
			require.Equal(t, "app1", process.AppID)
			require.Equal(t, "0", process.UserID)

			require.Equal(t, "test", app.Package.Driver)

			return "ext_proc1", nil
		},
	)

	nc.EXPECT().SetProcessRunning(gomock.Any()).Return(nil)

	err = startProcessExecutor.Execute(startProcessStateUpdate)
	assert.Nil(t, err)

	shouldExecute, err = startProcessExecutor.ShouldExecute(startProcessStateUpdate)
	assert.Nil(t, err)
	assert.Equal(t, false, shouldExecute)
}
*/
