package state

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

type JSONState struct {
	schema *NodeSchema
	state  []byte
}

func NewJSONState(schema *NodeSchema, initState []byte) (*JSONState, error) {
	err := schema.ValidateState(initState)
	if err != nil {
		return nil, fmt.Errorf("error validating initial state: %s", err)
	}

	return &JSONState{
		schema: schema,
		state:  initState,
	}, nil
}

func (s *JSONState) ApplyPatch(patchJSON []byte) error {
	updated, err := s.applyImpl(patchJSON)
	if err != nil {
		return err
	}

	s.state = updated
	return nil
}

func (s *JSONState) ValidatePatch(patchJSON []byte) ([]byte, error) {
	updated, err := s.applyImpl(patchJSON)
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
