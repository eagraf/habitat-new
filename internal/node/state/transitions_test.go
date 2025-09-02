package state

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTransitions(oldState *NodeState, transitions []Transition) (*NodeState, error) {
	var oldJSONState *JSONState
	schema := &NodeSchema{}
	if oldState == nil {
		emptyState, err := schema.EmptyState()
		if err != nil {
			return nil, err
		}
		ojs, err := emptyState.ToJSONState()
		if err != nil {
			return nil, err
		}
		oldJSONState = ojs
	} else {
		ojs, err := oldState.ToJSONState()
		if err != nil {
			return nil, err
		}

		oldJSONState = ojs
	}
	for _, t := range transitions {
		if t.Type() == "" {
			return nil, fmt.Errorf("transition type is empty")
		}

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

		newState, err := NewJSONState(schema, newStateBytes)
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

func TestFrontendDevMode(t *testing.T) {
	state, err := NewStateForLatestVersion()
	require.NoError(t, err)
	state.SetRootUserCert("fake_user_cert")
	newState, err := testTransitions(state, []Transition{
		&addReverseProxyRuleTransition{
			Rule: &ReverseProxyRule{
				Type:    ProxyRuleRedirect,
				ID:      "default-rule-frontend",
				Matcher: "", // Root matcher
			},
		},
	})
	require.Nil(t, err)

	frontendRule, ok := (newState.ReverseProxyRules)["default-rule-frontend"]
	require.Equal(t, true, ok)
	require.Equal(t, ProxyRuleRedirect, frontendRule.Type)
}

// use this if you expect the transitions to cause an error
func testTransitionsOnCopy(oldState *NodeState, transitions []Transition) (*NodeState, error) {
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
	transitions := []Transition{
		&initalizationTransition{
			InitState: &NodeState{
				NodeID:        "abc",
				Certificate:   "123",
				Name:          "New Node",
				SchemaVersion: CurrentVersion,
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

	_, err = testTransitions(nil, []Transition{
		&initalizationTransition{
			InitState: nil,
		},
	})
	assert.NotNil(t, err)
}

func TestAddingUsers(t *testing.T) {
	transitions := []Transition{
		&initalizationTransition{
			InitState: &NodeState{
				NodeID:        "abc",
				Certificate:   "123",
				Name:          "New Node",
				SchemaVersion: CurrentVersion,
			},
		},
		CreateAddUserTransition("eagraf", "fake-did"),
	}

	newState, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.NotNil(t, newState)
	assert.Equal(t, "abc", newState.NodeID)
	assert.Equal(t, "123", newState.Certificate)
	assert.Equal(t, "New Node", newState.Name)
	assert.Equal(t, 1, len(newState.Users))

	testSecondUserConflictOnUsername := []Transition{
		CreateAddUserTransition("eagraf", "fake-did"),
	}

	_, err = testTransitionsOnCopy(newState, testSecondUserConflictOnUsername)
	assert.NotNil(t, err)
}
func TestAppLifecycle(t *testing.T) {
	createTransition, _ := CreateStartInstallationTransition(
		"123",
		&Package{
			Driver:             DriverTypeDocker,
			RegistryURLBase:    "https://registry.com",
			RegistryPackageID:  "app_name1",
			RegistryPackageTag: "v1",
			DriverConfig:       map[string]interface{}{},
		},
		"1",
		"app_name1",
		[]*ReverseProxyRule{},
	)
	transitions := []Transition{
		&initalizationTransition{
			InitState: &NodeState{
				NodeID:        "abc",
				Certificate:   "123",
				Name:          "New Node",
				SchemaVersion: LatestVersion,
				Users: map[string]*User{
					"123": {
						ID:       "123",
						Username: "eagraf",
					},
				},
			},
		},
		createTransition,
	}

	newState, err := testTransitions(nil, transitions)
	require.Nil(t, err)
	require.NotNil(t, newState)
	assert.Equal(t, "abc", newState.NodeID)
	assert.Equal(t, "123", newState.Certificate)
	assert.Equal(t, "New Node", newState.Name)
	assert.Equal(t, 1, len(newState.Users))
	assert.Equal(t, 1, len(newState.AppInstallations))

	apps, err := newState.GetAppsForUser("123")
	assert.Nil(t, err)

	app := apps[0]
	_, ok := newState.AppInstallations[app.ID]
	assert.Equal(t, ok, true)
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, "app_name1", app.Name)
	assert.Equal(t, AppLifecycleStateInstalling, app.State)

	transition, _ := CreateStartInstallationTransition(
		"123",
		&Package{
			Driver:             DriverTypeDocker,
			RegistryURLBase:    "https://registry.com",
			RegistryPackageID:  "app_name1",
			RegistryPackageTag: "v1",
			DriverConfig:       map[string]interface{}{},
		},
		"1",
		"app_name1",
		[]*ReverseProxyRule{},
	)
	testSecondAppConflict := []Transition{
		transition,
	}

	_, err = testTransitionsOnCopy(newState, testSecondAppConflict)
	assert.NotNil(t, err)

	testInstallationCompleted := []Transition{
		&finishInstallationTransition{
			AppID: app.ID,
		},
	}

	newState, err = testTransitionsOnCopy(newState, testInstallationCompleted)
	assert.Nil(t, err)
	assert.Equal(t, AppLifecycleStateInstalled, newState.AppInstallations[app.ID].State)

	testUserDoesntExist := []Transition{
		&startInstallationTransition{
			AppInstallation: &AppInstallation{
				UserID:  "456",
				Name:    "app_name1",
				Version: "1",
				Package: &Package{
					Driver:             DriverTypeDocker,
					RegistryURLBase:    "https://registry.com",
					RegistryPackageID:  "app_name1",
					RegistryPackageTag: "v1",
				},
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testUserDoesntExist)
	assert.NotNil(t, err)

	trns, _ := CreateStartInstallationTransition(
		"123",
		&Package{
			Driver:             DriverTypeDocker,
			RegistryURLBase:    "https://registry.com",
			RegistryPackageID:  "app_name1",
			RegistryPackageTag: "v2",
			DriverConfig:       map[string]interface{}{},
		},
		"2",
		"app_name1",
		[]*ReverseProxyRule{},
	)
	testDifferentVersion := []Transition{
		trns,
	}
	_, err = testTransitionsOnCopy(newState, testDifferentVersion)
	assert.NotNil(t, err)

	testFinishOnAppThatsNotInstalling := []Transition{
		&finishInstallationTransition{
			AppID: "app_name1", // already installed
		},
	}

	_, err = testTransitionsOnCopy(newState, testFinishOnAppThatsNotInstalling)
	assert.NotNil(t, err)
}

func TestAppInstallReverseProxyRules(t *testing.T) {
	proxyRules := make(map[string]*ReverseProxyRule)
	transitions := []Transition{
		&initalizationTransition{
			InitState: &NodeState{
				NodeID:        "abc",
				Certificate:   "123",
				Name:          "New Node",
				SchemaVersion: LatestVersion,
				Users: map[string]*User{
					"123": {
						Username: "eagraf",
					},
				},
				ReverseProxyRules: proxyRules,
			},
		},
		&startInstallationTransition{
			AppInstallation: &AppInstallation{
				UserID:  "123",
				Name:    "app_name1",
				Version: "1",
				State:   AppLifecycleStateInstalled,
				Package: &Package{
					Driver:             DriverTypeDocker,
					RegistryURLBase:    "https://registry.com",
					RegistryPackageID:  "app_name1",
					RegistryPackageTag: "v1",
					DriverConfig:       map[string]interface{}{},
				},
			},
			NewProxyRules: []*ReverseProxyRule{
				{
					AppID:   "app1",
					Matcher: "/path",
					Target:  "http://localhost:8080",
					Type:    "redirect",
				},
			},
		},
	}

	newState, err := testTransitions(nil, transitions)
	require.Nil(t, err)
	require.NotNil(t, newState)
}

func TestProcesses(t *testing.T) {
	init := []Transition{
		&initalizationTransition{
			InitState: &NodeState{
				NodeID:        "abc",
				Certificate:   "123",
				Name:          "New Node",
				SchemaVersion: LatestVersion,
				Users: map[string]*User{
					"123": {
						ID:       "123",
						Username: "eagraf",
					},
				},
				AppInstallations: map[string]*AppInstallation{
					"App1": {
						ID:      "App1",
						Name:    "app_name1",
						UserID:  "123",
						Version: "1",
						State:   AppLifecycleStateInstalled,
						Package: &Package{
							Driver:             DriverTypeDocker,
							RegistryURLBase:    "https://registry.com",
							RegistryPackageID:  "app_name1",
							RegistryPackageTag: "v1",
							DriverConfig:       map[string]interface{}{},
						},
					},
				},
			},
		},
	}
	oldState, err := testTransitions(nil, init)
	require.NoError(t, err)

	startTransition, id, err := CreateProcessStartTransition("App1", oldState)
	require.NoError(t, err)

	newState, err := testTransitions(oldState, []Transition{startTransition})
	require.Nil(t, err)
	require.NotNil(t, newState)
	assert.Equal(t, "abc", newState.NodeID)
	assert.Equal(t, "123", newState.Certificate)
	assert.Equal(t, "New Node", newState.Name)
	assert.Equal(t, 1, len(newState.Users))
	assert.Equal(t, 1, len(newState.AppInstallations))
	assert.Equal(t, "app_name1", newState.AppInstallations["App1"].Name)
	assert.Equal(t, 1, len(newState.Processes))

	procs, err := newState.GetProcessesForUser("123")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(procs))

	proc := procs[0]
	assert.Equal(t, proc.ID, id)
	assert.Equal(t, "App1", proc.AppID)
	assert.Equal(t, "123", proc.UserID)

	testProcessRunningNoMatchingID := []Transition{
		&processStartTransition{
			Process: &Process{
				AppID: proc.AppID,
			},
		},
	}
	_, err = testTransitionsOnCopy(newState, testProcessRunningNoMatchingID)
	assert.NotNil(t, err)

	testAppIDConflict := []Transition{
		&processStartTransition{
			Process: &Process{
				AppID: "App1",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testAppIDConflict)
	assert.NotNil(t, err)

	// Test stopping the app
	newState, err = testTransitionsOnCopy(newState, []Transition{
		&processStopTransition{
			ProcessID: proc.ID,
		},
	})

	assert.Nil(t, err)
	assert.Equal(t, 0, len(newState.Processes))

	testUserDoesntExist := []Transition{
		&processStartTransition{
			Process: &Process{
				ID:     "proc2",
				AppID:  "App1",
				UserID: "456",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testUserDoesntExist)
	assert.NotNil(t, err)

	testAppDoesntExist := []Transition{
		&processStartTransition{
			Process: &Process{
				ID:     "proc3",
				AppID:  "App2",
				UserID: "123",
			},
		},
	}

	_, err = testTransitionsOnCopy(newState, testAppDoesntExist)
	assert.NotNil(t, err)

	testProcessStopNoMatchingID := []Transition{
		&processStopTransition{
			ProcessID: "proc500",
		},
	}
	_, err = testTransitionsOnCopy(newState, testProcessStopNoMatchingID)
	assert.NotNil(t, err)
}

func TestMigrationsTransition(t *testing.T) {
	transitions := []Transition{
		&initalizationTransition{
			InitState: &NodeState{
				NodeID:        "abc",
				Certificate:   "123",
				Name:          "New Node",
				SchemaVersion: "v0.0.1",
				Users: map[string]*User{
					"123": {
						ID:       "123",
						Username: "eagraf",
					},
				},
			},
		},
		&migrationTransition{
			TargetVersion: "v0.0.2",
		},
	}

	newState, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.Equal(t, "v0.0.2", newState.SchemaVersion)
	assert.Equal(t, "test", newState.TestField)

	testRemovingField := []Transition{
		&migrationTransition{
			TargetVersion: "v0.0.3",
		},
	}

	newState, err = testTransitionsOnCopy(newState, testRemovingField)
	assert.Nil(t, err)
	assert.Equal(t, "v0.0.3", newState.SchemaVersion)
	assert.Equal(t, "", newState.TestField)

	// Test migrating down
	testDown := []Transition{
		&migrationTransition{
			TargetVersion: "v0.0.1",
		},
	}
	newState, err = testTransitionsOnCopy(newState, testDown)
	assert.Nil(t, err)
	assert.Equal(t, "v0.0.1", newState.SchemaVersion)
	assert.Equal(t, "", newState.TestField)
}
