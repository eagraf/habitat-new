package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/qri-io/jsonschema"
	"github.com/rs/zerolog/log"
)

func keyError(errs []jsonschema.KeyError) error {
	s := strings.Builder{}
	for _, e := range errs {
		s.WriteString(fmt.Sprintf("%s\n", e.Error()))
	}
	return errors.New(s.String())
}

type Replicator interface {
	Dispatch([]byte) (*JSONState, error)
	UpdateChannel() <-chan StateUpdate
}

type State interface {
	Schema() []byte
	Bytes() ([]byte, error)
}

func StateToJSONState(state State) (*JSONState, error) {
	stateBytes, err := state.Bytes()
	if err != nil {
		return nil, err
	}
	jsonState, err := NewJSONState(state.Schema(), stateBytes)
	if err != nil {
		return nil, err
	}

	return jsonState, nil
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
	Bytes() []byte
	ProposeTransitions(transitions []Transition) (*JSONState, error)
}

type StateMachine struct {
	databaseID string
	jsonState  *JSONState // this JSONState is maintained in addition to
	publisher  pubsub.Publisher[StateUpdate]
	replicator Replicator
	updateChan <-chan StateUpdate
	doneChan   chan bool

	schema []byte
}

func NewStateMachine(databaseID string, schema, initRawState []byte, replicator Replicator, publisher pubsub.Publisher[StateUpdate]) (StateMachineController, error) {
	jsonState, err := NewJSONState(schema, initRawState)
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
				jsonState, err := NewJSONState(sm.schema, stateUpdate.NewState)
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

func (sm *StateMachine) Bytes() []byte {
	return sm.jsonState.Bytes()
}

// ProposeTransitions takes a list of transitions and applies them to the current state
// The hypothetical new state is returned. Importantly, this does not block until the state
// is "officially updated".
func (sm *StateMachine) ProposeTransitions(transitions []Transition) (*JSONState, error) {

	jsonStateBranch, err := sm.jsonState.Copy()
	if err != nil {
		return nil, err
	}

	wrappers := make([]*TransitionWrapper, 0)

	for _, t := range transitions {

		err = t.Validate(jsonStateBranch)
		if err != nil {
			return nil, fmt.Errorf("transition validation failed: %s", err)
		}

		patch, err := t.Patch(jsonStateBranch)
		if err != nil {
			return nil, err
		}

		err = jsonStateBranch.ApplyPatch(patch)
		if err != nil {
			return nil, err
		}

		wrapped, err := wrapTransition(t, jsonStateBranch)
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

func (sm *StateMachine) State() (*JSONState, error) {
	return sm.jsonState.Copy()
}
