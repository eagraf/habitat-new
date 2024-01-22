package state

import (
	"encoding/json"
)

const (
	TransitionTypeInitializeCommunity = "initialize_community"

	TransitionTypeInitializeCounter = "initialize_counter"
	TransitionTypeIncrementCounter  = "increment_counter"
)

type TransitionWrapper struct {
	Type       string `json:"type"`
	Patch      []byte `json:"patch"`      // The JSON patch generated from the transition struct
	Transition []byte `json:"transition"` // JSON encoded transition struct
}

type Transition interface {
	Type() string
	Patch(oldState *JSONState) ([]byte, error)
	Validate(oldState *JSONState) error
}

func wrapTransition(t Transition, oldState *JSONState) (*TransitionWrapper, error) {
	patch, err := t.Patch(oldState)
	if err != nil {
		return nil, err
	}

	transition, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	return &TransitionWrapper{
		Type:       t.Type(),
		Patch:      patch,
		Transition: transition,
	}, nil
}
