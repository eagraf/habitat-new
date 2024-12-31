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
	//IsLeader() bool
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

const MaxUint64 = ^uint64(0)

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

				// Only apply state updates if the update index is greater than the restart index.
				// If the update index is equal to the restart index, then the state update is a
				// restore message which tells the subscribers to restore everything from the most up to date state.
				if sm.restartIndex > stateUpdate.Index() && sm.restartIndex != MaxUint64 {
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
	stateFilePath string
	mutex         sync.Mutex

	updateChan chan hdb.StateUpdate
	schema     hdb.Schema
	jsonState  *hdb.JSONState
}

func NewStableStorageOnlyReplicator(stateFilePath string, schema hdb.Schema) (*StableStorageOnlyReplicator, error) {
	var jsonState *hdb.JSONState
	emptyState, err := schema.EmptyState()
	if err != nil {
		return nil, err
	}
	emptyStateBytes, err := emptyState.Bytes()
	if err != nil {
		return nil, err
	}
	jsonState, err = hdb.NewJSONState(schema, emptyStateBytes)
	if err != nil {
		return nil, err
	}

	return &StableStorageOnlyReplicator{
		stateFilePath: stateFilePath,
		mutex:         sync.Mutex{},

		jsonState:  jsonState,
		updateChan: make(chan hdb.StateUpdate),
		schema:     schema,
	}, nil
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

	return uint64(len(state.LogEntries)) - 1, nil
}

func (r *StableStorageOnlyReplicator) emitUpdate(log []byte, index uint64) {
	logB64 := []byte(base64.StdEncoding.EncodeToString(log))

	r.apply(&raft.Log{
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
	}()
	return nil
}

func (r *StableStorageOnlyReplicator) UpdateChannel() <-chan hdb.StateUpdate {
	return r.updateChan
}

// Dispatch takes a transition and applies it to the state machine.
// Note that since no replication takes place in this implementation, it is just persisted to stable storage.
func (r *StableStorageOnlyReplicator) Dispatch(transition []byte) (*hdb.JSONState, error) {
	newIndex, err := r.persistEntry(transition)
	if err != nil {
		return nil, err
	}

	r.emitUpdate(transition, newIndex)

	return r.jsonState, nil
}

// GetLastCommandIndex returns the index of the last log entry that was a command applied to the state machine.
func (r *StableStorageOnlyReplicator) GetLastCommandIndex() (uint64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	state, err := r.loadState()
	if err != nil {
		return 0, err
	}

	return uint64(len(state.LogEntries)) - 1, nil
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (r *StableStorageOnlyReplicator) apply(entry *raft.Log) interface{} {
	buf, err := base64.StdEncoding.DecodeString(string(entry.Data))
	if err != nil {
		log.Error().Msgf("error decoding log entry data: %s", err)
	}

	var wrappers []*hdb.TransitionWrapper
	err = json.Unmarshal(buf, &wrappers)
	if err != nil {
		log.Error().Msgf("error unmarshaling transition wrapper: %s", err)
	}

	for _, w := range wrappers {
		err = r.jsonState.ApplyPatch(w.Patch)
		if err != nil {
			log.Error().Msgf("error applying patch: %s", err)
		}

		metadata := hdb.NewStateUpdateMetadata(entry.Index, r.schema.Name())

		update, err := StateUpdateInternalFactory(r.schema.Name(), r.jsonState.Bytes(), w, metadata)
		if err != nil {
			log.Error().Msgf("error creating state update internal: %s", err)
		}

		r.updateChan <- update
	}

	return r.jsonState
}
