package state

import (
	"encoding/json"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/rs/zerolog/log"
)

type Replicator interface {
	Dispatch([]byte) (*hdb.JSONState, error)
	UpdateChannel() <-chan hdb.StateUpdate
	LastIndex() uint64
}

type StateMachineController interface {
	StartListening()
	StopListening()
	DatabaseID() string
	Bytes() []byte
	ProposeTransitions(transitions []hdb.Transition) (*hdb.JSONState, error)
}

type StateMachine struct {
	restartIndex uint64
	databaseID   string
	jsonState    *hdb.JSONState // this JSONState is maintained in addition to
	publisher    pubsub.Publisher[hdb.StateUpdate]
	replicator   Replicator
	updateChan   <-chan hdb.StateUpdate
	doneChan     chan bool

	schema []byte
}

func NewStateMachine(databaseID string, schema, initRawState []byte, replicator Replicator, publisher pubsub.Publisher[hdb.StateUpdate]) (StateMachineController, error) {
	jsonState, err := hdb.NewJSONState(schema, initRawState)
	if err != nil {
		return nil, err
	}
	return &StateMachine{
		restartIndex: replicator.LastIndex(),
		databaseID:   databaseID,
		jsonState:    jsonState,
		updateChan:   replicator.UpdateChannel(),
		replicator:   replicator,
		doneChan:     make(chan bool),
		publisher:    publisher,
		schema:       schema,
	}, nil
}

func (sm *StateMachine) StartListening() {
	go func() {
		for {
			select {
			case stateUpdate := <-sm.updateChan:
				// execute state update
				jsonState, err := hdb.NewJSONState(sm.schema, stateUpdate.NewState)
				if err != nil {
					log.Error().Err(err).Msgf("error getting new state from state update chan")
				}
				sm.jsonState = jsonState

				// When restoring the node, we ignore updates before the last index
				log.Info().Msgf("Restart index: %d, State update index: %d, Transition type: %s", sm.restartIndex, stateUpdate.Index, stateUpdate.TransitionType)
				if sm.restartIndex < stateUpdate.Index {
					err = sm.publisher.PublishEvent(&stateUpdate)
					if err != nil {
						log.Error().Err(err).Msgf("error publishing state update")
					}
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

		wrapped, err := hdb.WrapTransition(t, jsonStateBranch.Bytes())
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
