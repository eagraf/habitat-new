package node

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/stretchr/testify/assert"
)

func testTransitions(oldState *NodeState, transitions []hdb.Transition) (*NodeState, error) {
	var oldJSONState *hdb.JSONState
	schema := &NodeSchema{}
	if oldState == nil {
		emptyState, err := schema.InitState()
		if err != nil {
			return nil, err
		}
		ojs, err := hdb.StateToJSONState(emptyState)
		if err != nil {
			return nil, err
		}
		oldJSONState = ojs
	} else {
		ojs, err := hdb.StateToJSONState(oldState)
		if err != nil {
			return nil, err
		}

		oldJSONState = ojs
	}
	for _, t := range transitions {

		err := t.Validate(oldJSONState.Bytes())
		if err != nil {
			return nil, fmt.Errorf("transition validation failed: %s", err)
		}

		patch, err := t.Patch(oldJSONState.Bytes())
		if err != nil {
			return nil, err
		}

		newStateBytes, err := oldJSONState.ValidatePatch(patch)
		if err != nil {
			return nil, err
		}

		newState, err := hdb.NewJSONState(schema.Bytes(), newStateBytes)
		if err != nil {
			return nil, err
		}

		oldJSONState = newState
	}

	var state NodeState
	stateBytes := oldJSONState.Bytes()

	err := json.Unmarshal(stateBytes, &state)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

// use this if you expect the transitions to cause an error
func testTransitionsOnCopy(oldState *NodeState, transitions []hdb.Transition) (*NodeState, error) {
	marshaled, err := json.Marshal(oldState)
	if err != nil {
		return nil, err
	}

	var copy NodeState
	err = json.Unmarshal(marshaled, &copy)
	if err != nil {
		return nil, err
	}

	return testTransitions(&copy, transitions)
}

func TestNodeInitialization(t *testing.T) {
	transitions := []hdb.Transition{
		&InitalizationTransition{
			InitState: &NodeState{
				NodeID:      "abc",
				Certificate: "123",
				Name:        "New Node",
			},
		},
	}

	state, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "abc", state.NodeID)
	assert.Equal(t, "123", state.Certificate)
	assert.Equal(t, "New Node", state.Name)
	assert.Equal(t, 0, len(state.Users))
	assert.Equal(t, 0, len(state.Processes))

	_, err = testTransitions(nil, []hdb.Transition{
		&InitalizationTransition{
			InitState: nil,
		},
	})
	assert.NotNil(t, err)
}

func TestAddingUsers(t *testing.T) {
	transitions := []hdb.Transition{
		&InitalizationTransition{
			InitState: &NodeState{
				NodeID:      "abc",
				Certificate: "123",
				Name:        "New Node",
			},
		},
		&AddUserTransition{
			UserID:      "123",
			Username:    "eagraf",
			Certificate: "placeholder",
		},
	}

	newState, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.NotNil(t, newState)
	assert.Equal(t, "abc", newState.NodeID)
	assert.Equal(t, "123", newState.Certificate)
	assert.Equal(t, "New Node", newState.Name)
	assert.Equal(t, 1, len(newState.Users))

	testSecondUserConflictOnUsername := []hdb.Transition{
		&AddUserTransition{
			UserID:      "456",
			Username:    "eagraf",
			Certificate: "placeholder",
		},
	}

	_, err = testTransitionsOnCopy(newState, testSecondUserConflictOnUsername)
	assert.NotNil(t, err)

	testSecondUserConflictOnUserID := []hdb.Transition{
		&AddUserTransition{
			UserID:      "123",
			Username:    "eagraf2",
			Certificate: "placeholder",
		},
	}

	_, err = testTransitionsOnCopy(newState, testSecondUserConflictOnUserID)
	assert.NotNil(t, err)
}

func TestAppLifecycle(t *testing.T) {
	transitions := []hdb.Transition{
		&InitalizationTransition{
			InitState: &NodeState{
				NodeID:      "abc",
				Certificate: "123",
				Name:        "New Node",
				Users:       make([]*User, 0),
			},
		},
		&AddUserTransition{
			UserID:      "123",
			Username:    "eagraf",
			Certificate: "placeholder",
		},
		&StartInstallationTransition{
			UserID: "123",
			AppInstallation: &AppInstallation{
				Name:    "app_name1",
				Version: "1",
				Package: Package{
					Driver:             "docker",
					RegistryURLBase:    "https://registry.com",
					RegistryPackageID:  "app_name1",
					RegistryPackageTag: "v1",
				},
			},
		},
	}

	newState, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.NotNil(t, newState)
	assert.Equal(t, "abc", newState.NodeID)
	assert.Equal(t, "123", newState.Certificate)
	assert.Equal(t, "New Node", newState.Name)
	assert.Equal(t, 1, len(newState.Users))
	assert.Equal(t, 1, len(newState.Users[0].AppInstallations))
	assert.Equal(t, "app_name1", newState.Users[0].AppInstallations[0].Name)
	assert.Equal(t, "installing", newState.Users[0].AppInstallations[0].State)

	testSecondAppConflict := []hdb.Transition{
		&StartInstallationTransition{
			UserID: "123",
			AppInstallation: &AppInstallation{
				Version: "1",
				Package: Package{
					Driver:             "docker",
					RegistryURLBase:    "https://registry.com",
					RegistryPackageID:  "app_name1",
					RegistryPackageTag: "v1",
				},
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testSecondAppConflict)
	assert.NotNil(t, err)

	testUserDoesntExistOnFinish := []hdb.Transition{
		&FinishInstallationTransition{
			UserID:          "456",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
		},
	}
	_, err = testTransitionsOnCopy(newState, testUserDoesntExistOnFinish)
	assert.NotNil(t, err)

	testInstallationCompleted := []hdb.Transition{
		&FinishInstallationTransition{
			UserID:          "123",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
		},
	}

	newState, err = testTransitionsOnCopy(newState, testInstallationCompleted)
	assert.Nil(t, err)
	assert.Equal(t, "installed", newState.Users[0].AppInstallations[0].State)

	testUserDoesntExist := []hdb.Transition{
		&StartInstallationTransition{
			UserID: "456",
			AppInstallation: &AppInstallation{
				Name:    "app_name1",
				Version: "1",
				Package: Package{
					Driver:             "docker",
					RegistryURLBase:    "https://registry.com",
					RegistryPackageID:  "app_name1",
					RegistryPackageTag: "v1",
				},
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testUserDoesntExist)
	assert.NotNil(t, err)

	testDifferentVersion := []hdb.Transition{
		&StartInstallationTransition{
			UserID: "123",
			AppInstallation: &AppInstallation{
				Name:    "app_name1",
				Version: "2",
				Package: Package{
					Driver:             "docker",
					RegistryURLBase:    "https://registry.com",
					RegistryPackageID:  "app_name1",
					RegistryPackageTag: "v2",
				},
			},
		},
	}
	_, err = testTransitionsOnCopy(newState, testDifferentVersion)
	assert.NotNil(t, err)

	testFinishOnAppThatsNotInstalling := []hdb.Transition{
		&FinishInstallationTransition{
			UserID:          "123",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
		},
	}

	_, err = testTransitionsOnCopy(newState, testFinishOnAppThatsNotInstalling)
	assert.NotNil(t, err)
}

func TestProcesses(t *testing.T) {

	transitions := []hdb.Transition{
		&InitalizationTransition{
			InitState: &NodeState{
				NodeID:      "abc",
				Certificate: "123",
				Name:        "New Node",
				Users: []*User{
					{
						ID:          "123",
						Username:    "eagraf",
						Certificate: "placeholder",
						AppInstallations: []*AppInstallationState{
							{
								AppInstallation: &AppInstallation{
									ID:      "App1",
									Name:    "app_name1",
									Version: "1",
									Package: Package{
										Driver:             "docker",
										RegistryURLBase:    "https://registry.com",
										RegistryPackageID:  "app_name1",
										RegistryPackageTag: "v1",
									},
								},
								State: "installed",
							},
						},
					},
				},
			},
		},
		&ProcessStartTransition{
			Process: &Process{
				ID:     "proc1",
				AppID:  "App1",
				UserID: "123",
			},
		},
	}

	newState, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.NotNil(t, newState)
	assert.Equal(t, "abc", newState.NodeID)
	assert.Equal(t, "123", newState.Certificate)
	assert.Equal(t, "New Node", newState.Name)
	assert.Equal(t, 1, len(newState.Users))
	assert.Equal(t, 1, len(newState.Users[0].AppInstallations))
	assert.Equal(t, "app_name1", newState.Users[0].AppInstallations[0].Name)
	assert.Equal(t, "installed", newState.Users[0].AppInstallations[0].State)
	assert.Equal(t, 1, len(newState.Processes))
	assert.Equal(t, "proc1", newState.Processes[0].ID)
	assert.Equal(t, "starting", newState.Processes[0].State)
	assert.Equal(t, "App1", newState.Processes[0].AppID)
	assert.Equal(t, "123", newState.Processes[0].UserID)

	// The app moves to running state

	testProcessRunning := []hdb.Transition{
		&ProcessRunningTransition{
			ProcessID:   "proc1",
			ExtDriverID: "docker_container_1",
		},
	}

	newState, err = testTransitionsOnCopy(newState, testProcessRunning)
	assert.Nil(t, err)
	assert.Equal(t, "running", newState.Processes[0].State)
	assert.Equal(t, "docker_container_1", newState.Processes[0].ExtDriverID)

	testProcessRunningNoMatchingID := []hdb.Transition{
		&ProcessRunningTransition{
			ProcessID:   "proc2",
			ExtDriverID: "docker_container_1",
		},
	}
	_, err = testTransitionsOnCopy(newState, testProcessRunningNoMatchingID)
	assert.NotNil(t, err)

	testProcessIDConflict := []hdb.Transition{
		&ProcessStartTransition{
			Process: &Process{
				ID:     "proc1",
				AppID:  "App1",
				UserID: "123",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testProcessIDConflict)
	assert.NotNil(t, err)

	testProcessAlreadyRunning := []hdb.Transition{
		&ProcessRunningTransition{
			ProcessID:   "proc1",
			ExtDriverID: "docker_container_1",
		},
	}

	_, err = testTransitionsOnCopy(newState, testProcessAlreadyRunning)
	assert.NotNil(t, err)

	testAppIDConflict := []hdb.Transition{
		&ProcessStartTransition{
			Process: &Process{
				ID:     "proc2",
				AppID:  "App1",
				UserID: "123",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testAppIDConflict)
	assert.NotNil(t, err)

	// Test stopping the app
	newState, err = testTransitionsOnCopy(newState, []hdb.Transition{
		&ProcessStopTransition{
			ProcessID: "proc1",
		},
	})

	assert.Nil(t, err)
	assert.Equal(t, 0, len(newState.Processes))

	testUserDoesntExist := []hdb.Transition{
		&ProcessStartTransition{
			Process: &Process{
				ID:     "proc2",
				AppID:  "App1",
				UserID: "456",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testUserDoesntExist)
	assert.NotNil(t, err)

	testAppDoesntExist := []hdb.Transition{
		&ProcessStartTransition{
			Process: &Process{
				ID:     "proc3",
				AppID:  "App2",
				UserID: "123",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testAppDoesntExist)
	assert.NotNil(t, err)

	testProcessStopNoMatchingID := []hdb.Transition{
		&ProcessStopTransition{
			ProcessID: "proc500",
		},
	}
	_, err = testTransitionsOnCopy(newState, testProcessStopNoMatchingID)
	assert.NotNil(t, err)

}
