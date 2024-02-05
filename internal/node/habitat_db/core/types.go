package core

import (
	"encoding/json"
	"reflect"
)

type Schema interface {
	Name() string
	InitState() (State, error)
	Bytes() []byte
	Type() reflect.Type
	InitializationTransition(initState []byte) (Transition, error)
}

type TransitionWrapper struct {
	Type       string `json:"type"`
	Patch      []byte `json:"patch"`      // The JSON patch generated from the transition struct
	Transition []byte `json:"transition"` // JSON encoded transition struct
}

type Transition interface {
	Type() string
	Patch(oldState []byte) ([]byte, error)
	Validate(oldState []byte) error
}

func WrapTransition(t Transition, oldState []byte) (*TransitionWrapper, error) {
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

type Client interface {
	DatabaseID() string
	ProposeTransitions(transitions []Transition) (*JSONState, error)
	Bytes() []byte
}
