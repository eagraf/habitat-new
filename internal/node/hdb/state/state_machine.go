package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/pubsub"

	"github.com/rs/zerolog/log"
)

type StateMachineController interface {
	StartListening(context.Context)
	StopListening()
	DatabaseID() string
	Bytes() []byte
	ProposeTransitions(transitions []hdb.Transition) (*hdb.JSONState, error)
}

type StateMachine struct {
	databaseID string
	jsonState  *hdb.JSONState // this JSONState is maintained in addition to
	publisher  pubsub.Publisher[hdb.StateUpdate]
	updateChan <-chan hdb.StateUpdate
	doneChan   chan bool
	writeToDB  func([]byte) error

	schema hdb.Schema
}

func NewStateMachine(databaseID string, schema hdb.Schema, initRawState []byte, publisher pubsub.Publisher[hdb.StateUpdate], writeToDB func([]byte) error) (StateMachineController, error) {
	jsonState, err := hdb.NewJSONState(schema, initRawState)
	if err != nil {
		return nil, err
	}

	return &StateMachine{
		databaseID: databaseID,
		jsonState:  jsonState,
		updateChan: make(<-chan hdb.StateUpdate),
		doneChan:   make(chan bool),
		publisher:  publisher,
		schema:     schema,
		writeToDB:  writeToDB,
	}, nil
}

func (sm *StateMachine) StartListening(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		// TODO this bit of code should be well tested
		case stateUpdate := <-sm.updateChan:
			// execute state update
			stateBytes, err := stateUpdate.NewState().Bytes()
			if err != nil {
				log.Error().Err(err).Msgf("error getting new state bytes from state update chan")
			}
			jsonState, err := hdb.NewJSONState(sm.schema, stateBytes)
			if err != nil {
				log.Error().Err(err).Msgf("error getting new state from state update chan")
			}
			sm.jsonState = jsonState

			err = sm.publisher.PublishEvent(stateUpdate)
			if err != nil {
				log.Error().Err(err).Msgf("error publishing state update")
			}
		case <-sm.doneChan:
			return
		}
	}
}

func (sm *StateMachine) StopListening() {
	sm.doneChan <- true
}

func (sm *StateMachine) DatabaseID() string {
	return sm.databaseID
}

func (sm *StateMachine) Bytes() []byte {
	return sm.jsonState.Bytes()
}

// ProposeTransitions takes a list of transitions and applies them to the current state
// The hypothetical new state is returned. Importantly, this does not block until the state
// is "officially updated".
func (sm *StateMachine) ProposeTransitions(transitions []hdb.Transition) (*hdb.JSONState, error) {
	jsonStateBranch, err := sm.jsonState.Copy()
	if err != nil {
		return nil, err
	}

	wrappers := make([]*hdb.TransitionWrapper, 0)

	for _, t := range transitions {
		err = t.Validate(jsonStateBranch.Bytes())
		if err != nil {
			return nil, fmt.Errorf("transition validation failed: %s", err)
		}

		patch, err := t.Patch(jsonStateBranch.Bytes())
		if err != nil {
			return nil, err
		}

		err = jsonStateBranch.ApplyPatch(patch)
		if err != nil {
			return nil, err
		}

		wrapped, err := hdb.WrapTransition(t, patch, jsonStateBranch.Bytes())
		if err != nil {
			return nil, err
		}

		wrappers = append(wrappers, wrapped)
	}

	transitionsJSON, err := json.Marshal(wrappers)
	if err != nil {
		return nil, err
	}
	log.Info().Msg(string(transitionsJSON))

	// Write to the database
	return jsonStateBranch, sm.writeToDB(jsonStateBranch.Bytes())
}
