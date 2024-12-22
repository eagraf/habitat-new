package state

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/pubsub"
	"github.com/hashicorp/raft"

	"github.com/rs/zerolog/log"
)

type Replicator interface {
	Dispatch([]byte) (*hdb.JSONState, error)
	UpdateChannel() <-chan hdb.StateUpdate
	IsLeader() bool
	GetLastCommandIndex() (uint64, error)
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

	schema hdb.Schema
}

func NewStateMachine(databaseID string, schema hdb.Schema, initRawState []byte, replicator Replicator, publisher pubsub.Publisher[hdb.StateUpdate]) (StateMachineController, error) {
	jsonState, err := hdb.NewJSONState(schema, initRawState)
	if err != nil {
		return nil, err
	}

	restartIndex, err := replicator.GetLastCommandIndex()
	if err != nil {
		return nil, err
	}
	return &StateMachine{
		restartIndex: restartIndex,
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
			// TODO this bit of code should be well tested
			case stateUpdate := <-sm.updateChan:

				// Only publish state updates if this node is the leader node.
				if sm.replicator.IsLeader() {
					// Only apply state updates if the update index is greater than the restart index.
					// If the update index is equal to the restart index, then the state update is a
					// restore message which tells the subscribers to restore everything from the most up to date state.
					if sm.restartIndex > stateUpdate.Index() {
						continue
					}

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

					if sm.restartIndex == stateUpdate.Index() {
						log.Info().Msgf("Restoring node state")
						stateUpdate.SetRestore()
					}

					err = sm.publisher.PublishEvent(stateUpdate)
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

		err = t.Enrich(sm.jsonState.Bytes())
		if err != nil {
			return nil, fmt.Errorf("transition enrichment failed: %s", err)
		}

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

	_, err = sm.replicator.Dispatch(transitionsJSON)
	if err != nil {
		return nil, err
	}

	return jsonStateBranch, nil
}

// StableStorageLogEntry represents a single transition to the state machine, as it is written
// to stable storage.
type StableStorageLogEntry struct {
	CreatedAt time.Time `json:"created_at"`
	Entry     []byte    `json:"entry"`
}

// StableStorageState represents the state of the state machine as it is persisted to stable storage.
type StableStorageState struct {
	LogEntries []StableStorageLogEntry `json:"log_entries"`
}

// StableStorageOnlyReplicator is a bare minimum replicator that just persists state to a file.
type StableStorageOnlyReplicator struct {
	FSM           *RaftFSMAdapter
	stateFilePath string
	mutex         sync.Mutex
	lastIndex     uint64
}

func NewStableStorageOnlyReplicator(fsm *RaftFSMAdapter, stateFilePath string) *StableStorageOnlyReplicator {
	return &StableStorageOnlyReplicator{
		FSM:           fsm,
		stateFilePath: stateFilePath,
		mutex:         sync.Mutex{},
		lastIndex:     0,
	}
}

// InitializeState initializes stable storage for the state machine.
func (r *StableStorageOnlyReplicator) InitializeState() error {
	emptyState := &StableStorageState{
		LogEntries: []StableStorageLogEntry{},
	}

	stateBytes, err := json.Marshal(emptyState)
	if err != nil {
		return fmt.Errorf("error marshaling stable storage state: %s", err)
	}

	err = os.WriteFile(r.stateFilePath, stateBytes, 0600)
	if err != nil {
		return fmt.Errorf("error writing stable storage state file: %s", err)
	}

	r.lastIndex = 0

	return nil
}

// loadState loads the state of the state machine from stable storage.
func (r *StableStorageOnlyReplicator) loadState() (*StableStorageState, error) {
	stateBytes, err := os.ReadFile(r.stateFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading stable storage state file: %s", err)
	}

	var state StableStorageState
	err = json.Unmarshal(stateBytes, &state)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling stable storage state: %s", err)
	}

	return &state, nil
}

func (r *StableStorageOnlyReplicator) persistEntry(entry []byte) (uint64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	state, err := r.loadState()
	if err != nil {
		return 0, err
	}

	state.LogEntries = append(state.LogEntries, StableStorageLogEntry{
		CreatedAt: time.Now(),
		Entry:     entry,
	})

	stateBytes, err := json.Marshal(state)
	if err != nil {
		return 0, fmt.Errorf("error marshaling stable storage state: %s", err)
	}

	err = os.WriteFile(r.stateFilePath, stateBytes, 0600)
	if err != nil {
		return 0, fmt.Errorf("error writing stable storage state file: %s", err)
	}

	newIndex := r.lastIndex + 1
	r.lastIndex = newIndex

	return newIndex, nil
}

func (r *StableStorageOnlyReplicator) emitUpdate(log []byte, index uint64) {
	logB64 := []byte(base64.StdEncoding.EncodeToString(log))

	r.FSM.Apply(&raft.Log{
		Data:  logB64,
		Type:  raft.LogCommand,
		Index: index,
	})
}

func (r *StableStorageOnlyReplicator) RestoreState() error {
	go func() {
		r.mutex.Lock()
		defer r.mutex.Unlock()

		state, err := r.loadState()
		if err != nil {
			log.Fatal().Err(err).Msgf("error reloading state")
		}

		for i, entry := range state.LogEntries {
			r.emitUpdate(entry.Entry, uint64(i))
		}

		r.lastIndex = uint64(len(state.LogEntries))

	}()
	return nil
}

func (r *StableStorageOnlyReplicator) UpdateChannel() <-chan hdb.StateUpdate {
	return r.FSM.UpdateChan()
}

func (r *StableStorageOnlyReplicator) Dispatch(transition []byte) (*hdb.JSONState, error) {
	newIndex, err := r.persistEntry(transition)
	if err != nil {
		return nil, err
	}

	r.emitUpdate(transition, newIndex)

	return r.FSM.JSONState(), nil
}

func (r *StableStorageOnlyReplicator) IsLeader() bool {
	return true
}

// GetLastCommandIndex returns the index of the last log entry that was a command applied to the state machine.
func (r *StableStorageOnlyReplicator) GetLastCommandIndex() (uint64, error) {
	return r.lastIndex, nil
}
