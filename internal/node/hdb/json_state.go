package hdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/qri-io/jsonschema"
)

func keyError(errs []jsonschema.KeyError) error {
	s := strings.Builder{}
	for _, e := range errs {
		s.WriteString(fmt.Sprintf("%s\n", e.Error()))
	}
	return errors.New(s.String())
}

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

func (s *JSONState) ApplyPatch(patchJSON []byte) error {
	updated, err := s.applyImpl(patchJSON)
	if err != nil {
		return err
	}

	// only update state if everything worked out
	s.Lock()
	defer s.Unlock()

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
