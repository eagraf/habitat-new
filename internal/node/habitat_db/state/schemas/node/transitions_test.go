package node

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/stretchr/testify/assert"
)

func testTransitions(oldState *NodeState, transitions []state.Transition) (*NodeState, error) {
	var oldJSONState *state.JSONState
	schema := &NodeSchema{}
	if oldState == nil {
		emptyState, err := schema.InitState()
		if err != nil {
			return nil, err
		}
		ojs, err := state.StateToJSONState(emptyState)
		if err != nil {
			return nil, err
		}
		oldJSONState = ojs
	} else {
		ojs, err := state.StateToJSONState(oldState)
		if err != nil {
			return nil, err
		}

		oldJSONState = ojs
	}
	for _, t := range transitions {

		err := t.Validate(oldJSONState)
		if err != nil {
			return nil, fmt.Errorf("transition validation failed: %s", err)
		}

		patch, err := t.Patch(oldJSONState)
		if err != nil {
			return nil, err
		}

		newStateBytes, err := oldJSONState.ValidatePatch(patch)
		if err != nil {
			return nil, err
		}

		newState, err := state.NewJSONState(schema.Bytes(), newStateBytes)
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
func testTransitionsOnCopy(oldState *NodeState, transitions []state.Transition) (*NodeState, error) {
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
	transitions := []state.Transition{
		&InitalizationTransition{
			NodeID:      "abc",
			Certificate: "123",
			Name:        "New Node",
		},
	}

	state, err := testTransitions(nil, transitions)
	assert.Nil(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "abc", state.NodeID)
	assert.Equal(t, "123", state.Certificate)
	assert.Equal(t, "New Node", state.Name)
	assert.Equal(t, 0, len(state.Users))
}

func TestAddingUsers(t *testing.T) {
	transitions := []state.Transition{
		&InitalizationTransition{
			NodeID:      "abc",
			Certificate: "123",
			Name:        "New Node",
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

	testSecondUserConflictOnUsername := []state.Transition{
		&AddUserTransition{
			UserID:      "456",
			Username:    "eagraf",
			Certificate: "placeholder",
		},
	}

	newState, err = testTransitionsOnCopy(newState, testSecondUserConflictOnUsername)
	assert.NotNil(t, err)

	testSecondUserConflictOnUserID := []state.Transition{
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
	transitions := []state.Transition{
		&InitalizationTransition{
			NodeID:      "abc",
			Certificate: "123",
			Name:        "New Node",
		},
		&AddUserTransition{
			UserID:      "123",
			Username:    "eagraf",
			Certificate: "placeholder",
		},
		&StartInstallationTransition{
			UserID:          "123",
			Name:            "app_name1",
			Version:         "1",
			Driver:          "docker",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
			RegistryTag:     "v1",
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

	testSecondAppConflict := []state.Transition{
		&StartInstallationTransition{
			UserID:          "123",
			Version:         "1",
			Driver:          "docker",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
			RegistryTag:     "v1",
		},
	}

	_, err = testTransitionsOnCopy(newState, testSecondAppConflict)
	assert.NotNil(t, err)

	testInstallationCompleted := []state.Transition{
		&FinishInstallationTransition{
			UserID:          "123",
			RegistryURLBase: "https://registry.com",
			RegistryAppID:   "app_name1",
		},
	}

	newState, err = testTransitionsOnCopy(newState, testInstallationCompleted)
	assert.Nil(t, err)
	assert.Equal(t, "installed", newState.Users[0].AppInstallations[0].State)
}
