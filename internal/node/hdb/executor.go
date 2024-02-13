package hdb

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// StateUpdate includes all necessary information to update the state of an external system to match
// the state machine.
type StateUpdate struct {
	SchemaType     string
	DatabaseID     string
	NewState       []byte
	Transition     []byte
	TransitionType string
}

// IdempoentStateUpdateExecutor is an interface for a state update executor that will check
// if the desired external system state is already achieved before executing the action described
// by the state transition.
type IdempotentStateUpdateExecutor interface {
	TransitionType() string

	// ShouldExecute returns true if the state update should be executed.
	ShouldExecute(*StateUpdate) (bool, error)

	// Execute the given state update.
	Execute(*StateUpdate) error
}

// IdempotentStateUpdateSubscriber will run a IdempotentStateUpdateExecutor when it receives
// a state update for a matching transition.
type IdempotentStateUpdateSubscriber struct {
	name       string
	schemaName string
	executors  map[string]IdempotentStateUpdateExecutor
}

func NewIdempotentStateUpdateSubscriber(name, schemaName string, executors []IdempotentStateUpdateExecutor) (*IdempotentStateUpdateSubscriber, error) {

	res := &IdempotentStateUpdateSubscriber{
		name:       name,
		schemaName: schemaName,
		executors:  make(map[string]IdempotentStateUpdateExecutor),
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
	if s.schemaName != event.SchemaType {
		// This is a no-op. We don't error since the pubsub system isn't sophisticated enough to
		// filter by topic.
		return nil
	}

	executor, ok := s.executors[event.TransitionType]
	if !ok {
		// This is a no-op. We don't error since the pubsub system isn't sophisticated enough to
		// filter by topic.
		return nil
	}

	shouldExecute, err := executor.ShouldExecute(event)
	if err != nil {
		return fmt.Errorf("Error checking if transition %s should execute: %w", event.TransitionType, err)
	}

	if !shouldExecute {
		// The desired state is already achieved.
		log.Info().Msgf("Desired state achieved idempotently for transition %s", event.TransitionType)
		return nil
	}

	return executor.Execute(event)
}