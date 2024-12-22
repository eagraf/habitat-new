package controller

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/hdb"
)

func testTransitions(oldState *node.State, transitions []hdb.Transition) (*node.State, error) {
	var prevState *hdb.JSONState
	schema := &node.NodeSchema{}
	if oldState == nil {
		emptyState, err := schema.EmptyState()
		if err != nil {
			return nil, err
		}
		ojs, err := hdb.StateToJSONState(emptyState)
		if err != nil {
			return nil, err
		}
		prevState = ojs
	} else {
		ojs, err := hdb.StateToJSONState(oldState)
		if err != nil {
			return nil, err
		}

		prevState = ojs
	}
	// Continuously update prevState with each transition
	for _, t := range transitions {
		if t.Type() == "" {
			return nil, fmt.Errorf("transition type is empty")
		}

		err := t.Enrich(prevState.Bytes())
		if err != nil {
			return nil, fmt.Errorf("transition enrichment failed: %s", err)
		}

		err = t.Validate(prevState.Bytes())
		if err != nil {
			return nil, fmt.Errorf("transition validation failed: %s", err)
		}

		patch, err := t.Patch(prevState.Bytes())
		if err != nil {
			return nil, err
		}

		newStateBytes, err := prevState.ValidatePatch(patch)
		if err != nil {
			return nil, err
		}

		newState, err := hdb.NewJSONState(schema, newStateBytes)
		if err != nil {
			return nil, err
		}

		prevState = newState
	}

	var state node.State
	stateBytes := prevState.Bytes()

	err := json.Unmarshal(stateBytes, &state)
	if err != nil {
		return nil, err
	}

	return &state, nil
}
