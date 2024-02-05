package state

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/core"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/rs/zerolog/log"
)

type Replicator interface {
	Dispatch([]byte) (*core.JSONState, error)
	UpdateChannel() <-chan StateUpdate
}

type StateUpdate struct {
	SchemaType     string
	DatabaseID     string
	NewState       []byte
	Transition     []byte
	TransitionType string
}

type StateMachineController interface {
	StartListening()
	StopListening()
	DatabaseID() string
	Bytes() []byte
	ProposeTransitions(transitions []core.Transition) (*core.JSONState, error)
}

type StateMachine struct {
	databaseID string
	jsonState  *core.JSONState // this JSONState is maintained in addition to
	publisher  pubsub.Publisher[StateUpdate]
	replicator Replicator
	updateChan <-chan StateUpdate
	doneChan   chan bool

	schema []byte
}

func NewStateMachine(databaseID string, schema, initRawState []byte, replicator Replicator, publisher pubsub.Publisher[StateUpdate]) (StateMachineController, error) {
	jsonState, err := core.NewJSONState(schema, initRawState)
	if err != nil {
		return nil, err
	}
	return &StateMachine{
		databaseID: databaseID,
		jsonState:  jsonState,
		updateChan: replicator.UpdateChannel(),
		replicator: replicator,
		doneChan:   make(chan bool),
		publisher:  publisher,
		schema:     schema,
	}, nil
}

func (sm *StateMachine) StartListening() {
	go func() {
		for {
			select {
			case stateUpdate := <-sm.updateChan:
				// execute state update
				jsonState, err := core.NewJSONState(sm.schema, stateUpdate.NewState)
				if err != nil {
					log.Error().Err(err).Msgf("error getting new state from state update chan")
				}
				sm.jsonState = jsonState
				err = sm.publisher.PublishEvent(&stateUpdate)
				if err != nil {
					log.Error().Err(err).Msgf("error publishing state update")
				}
			case <-sm.doneChan:
				return
			}
		}
	}()
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
func (sm *StateMachine) ProposeTransitions(transitions []core.Transition) (*core.JSONState, error) {

	jsonStateBranch, err := sm.jsonState.Copy()
	if err != nil {
		return nil, err
	}

	wrappers := make([]*core.TransitionWrapper, 0)

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

		wrapped, err := core.WrapTransition(t, jsonStateBranch.Bytes())
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

	_, err = sm.replicator.Dispatch(transitionsJSON)
	if err != nil {
		return nil, err
	}

	return jsonStateBranch, nil
}
