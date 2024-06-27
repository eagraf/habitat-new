package hdb

import (
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/rs/zerolog/log"
)

// StateUpdate includes all necessary information to update the state of an external system to match
// the state machine.
type StateUpdate struct {
	Index      uint64
	SchemaType string
	DatabaseID string
	Restore    bool

	StateUpdateInternal
}

type StateUpdateInternal interface {
	NewState() State
	Transition() []byte
	TransitionType() string
}

// IdempoentStateUpdateExecutor is an interface for a state update executor that will check
// if the desired external system state is already achieved before executing the action described
// by the state transition.
type IdempotentStateUpdateExecutor interface {
	TransitionType() string

	// ShouldExecute returns true if the state update should be executed.
	ShouldExecute(StateUpdateInternal) (bool, error)

	// Execute the given state update.
	Execute(StateUpdateInternal) error

	PostHook(StateUpdateInternal) error
}

type StateRestorer interface {
	Restore(StateUpdateInternal) error
}

type StateUpdateSubscriber interface {
	pubsub.Subscriber[StateUpdate]
	StateRestorer
}

// IdempotentStateUpdateSubscriber will run a IdempotentStateUpdateExecutor when it receives
// a state update for a matching transition.
type IdempotentStateUpdateSubscriber struct {
	name       string
	schemaName string
	executors  map[string]IdempotentStateUpdateExecutor
	StateRestorer
}

func NewIdempotentStateUpdateSubscriber(name, schemaName string, executors []IdempotentStateUpdateExecutor, stateRestorer StateRestorer) (*IdempotentStateUpdateSubscriber, error) {

	res := &IdempotentStateUpdateSubscriber{
		name:          name,
		schemaName:    schemaName,
		executors:     make(map[string]IdempotentStateUpdateExecutor),
		StateRestorer: stateRestorer,
	}

	for _, executor := range executors {
		if _, ok := res.executors[executor.TransitionType()]; ok {
			return nil, fmt.Errorf("duplicate executor for transition type %s", executor.TransitionType())
		}
		res.executors[executor.TransitionType()] = executor
	}

	return res, nil
}

func (s *IdempotentStateUpdateSubscriber) Name() string {
	return s.name
}

func (s *IdempotentStateUpdateSubscriber) ConsumeEvent(event *StateUpdate) error {
	// If the restore flag is set, we restore the entire state rather than processing the individual state update.
	if event.Restore {
		err := s.Restore(event)
		if err != nil {
			return fmt.Errorf("Error restoring node state: %w", err)
		}
		return nil
	}

	// Otherwise, process the state update.
	if s.schemaName != event.SchemaType {
		// This is a no-op. We don't error since the pubsub system isn't sophisticated enough to
		// filter by topic.
		return nil
	}

	executor, ok := s.executors[event.TransitionType()]
	if !ok {
		// This is a no-op. We don't error since the pubsub system isn't sophisticated enough to
		// filter by topic.
		return nil
	}

	shouldExecute, err := executor.ShouldExecute(event)
	if err != nil {
		return fmt.Errorf("Error checking if transition %s should execute: %w", event.TransitionType(), err)
	}

	if shouldExecute {
		err = executor.Execute(event)
		if err != nil {
			return fmt.Errorf("Error acting on state update for transition %s: %w", event.TransitionType(), err)
		}
	} else {
		// The desired state is already achieved.
		log.Info().Msgf("Desired state achieved idempotently for transition %s", event.TransitionType())
	}

	err = executor.PostHook(event)
	if err != nil {
		return fmt.Errorf("Error executing post-hook for transition %s: %s", event.TransitionType(), err)
	}

	return nil
}
