package hdb

import (
	"encoding/json"
	"fmt"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/wI2L/jsondiff"
)

type JSONState struct {
	schema Schema
	state  []byte

	*sync.Mutex
}

func NewJSONState(schema Schema, initState []byte) (*JSONState, error) {

	err := schema.ValidateState(initState)
	if err != nil {
		return nil, fmt.Errorf("error validating initial state: %s", err)
	}

	return &JSONState{
		schema: schema,
		state:  initState,
		Mutex:  &sync.Mutex{},
	}, nil
}

func (s *JSONState) ApplyPatch(patch jsondiff.Patch) error {
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	updated, err := s.applyImpl(patchBytes)
	if err != nil {
		return err
	}

	// only update state if everything worked out
	s.Lock()
	defer s.Unlock()

	s.state = updated

	return nil
}

func (s *JSONState) ValidatePatch(patch jsondiff.Patch) ([]byte, error) {
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	updated, err := s.applyImpl(patchBytes)
	if err != nil {
		return nil, err
	}

	return updated, err
}

func (s *JSONState) applyImpl(patchJSON []byte) ([]byte, error) {
	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON patch: %s", err)
	}
	updated, err := patch.Apply(s.state)
	if err != nil {
		return nil, fmt.Errorf("error applying patch to current state: %s", err)
	}

	// check that updated state still fulfills the schema
	err = s.schema.ValidateState(updated)
	if err != nil {
		return nil, fmt.Errorf("error validating updated state: %s", err)
	}
	return updated, nil
}

func (s *JSONState) Unmarshal(dest interface{}) error {
	return json.Unmarshal(s.state, dest)
}

func (s *JSONState) Bytes() []byte {
	return s.state
}

func (s *JSONState) Copy() (*JSONState, error) {
	return NewJSONState(s.schema, s.state)
}
