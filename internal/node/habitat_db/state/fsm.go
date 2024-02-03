package state

import (
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/hashicorp/raft"
	"github.com/rs/zerolog/log"
)

// RaftFSMAdapter makes our state machine struct play nice with the Raft library
// It treats all state as a JSON blob so that things are serializable, which makes it
// easy to decouple the state machine types from the Raft library
type RaftFSMAdapter struct {
	databaseID string
	jsonState  *JSONState
	updateChan chan StateUpdate
	schema     Schema
}

func NewRaftFSMAdapter(databaseID string, schema Schema, commState []byte) (*RaftFSMAdapter, error) {
	var jsonState *JSONState
	var err error
	if commState == nil {
		initState, err := schema.InitState()
		if err != nil {
			return nil, err
		}
		initStateBytes, err := initState.Bytes()
		if err != nil {
			return nil, err
		}
		jsonState, err = NewJSONState(schema.Bytes(), initStateBytes)
		if err != nil {
			return nil, err
		}
	} else {
		jsonState, err = NewJSONState(schema.Bytes(), commState)
		if err != nil {
			return nil, err
		}
	}

	return &RaftFSMAdapter{
		databaseID: databaseID,
		jsonState:  jsonState,
		updateChan: make(chan StateUpdate),
		schema:     schema,
	}, nil
}

func (sm *RaftFSMAdapter) JSONState() *JSONState {
	return sm.jsonState
}

func (sm *RaftFSMAdapter) UpdateChan() <-chan StateUpdate {
	return sm.updateChan
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (sm *RaftFSMAdapter) Apply(entry *raft.Log) interface{} {
	buf, err := base64.StdEncoding.DecodeString(string(entry.Data))
	if err != nil {
		log.Error().Msgf("error decoding log entry data: %s", err)
	}

	var wrappers []*TransitionWrapper
	err = json.Unmarshal(buf, &wrappers)
	if err != nil {
		log.Error().Msgf("error unmarshaling transition wrapper: %s", err)
	}

	for _, w := range wrappers {
		err = sm.jsonState.ApplyPatch(w.Patch)
		if err != nil {
			log.Error().Msgf("error applying patch: %s", err)
		}

		sm.updateChan <- StateUpdate{
			SchemaType:     sm.schema.Name(),
			DatabaseID:     sm.databaseID,
			TransitionType: w.Type,
			Transition:     w.Transition,
			NewState:       sm.jsonState.Bytes(),
		}
	}

	return sm.JSONState()
}

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (sm *RaftFSMAdapter) Snapshot() (raft.FSMSnapshot, error) {
	return &FSMSnapshot{
		state: sm.jsonState.Bytes(),
	}, nil
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (sm *RaftFSMAdapter) Restore(reader io.ReadCloser) error {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	state, err := NewJSONState(sm.schema.Bytes(), buf)
	if err != nil {
		return err
	}

	sm.jsonState = state
	return nil
}

func (sm *RaftFSMAdapter) State() ([]byte, error) {
	return sm.jsonState.Bytes(), nil
}

type FSMSnapshot struct {
	state []byte
}

// Persist should dump all necessary state to the WriteCloser 'sink',
// and call sink.Close() when finished or call sink.Cancel() on error.
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	sink.Write(s.state)
	return sink.Close()
}

// Release is invoked when we are finished with the snapshot.
func (s *FSMSnapshot) Release() {
}
