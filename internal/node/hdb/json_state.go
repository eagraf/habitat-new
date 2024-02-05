package hdb

import (
	"context"
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
	schema *jsonschema.Schema
	state  []byte

	*sync.Mutex
}

func NewJSONState(jsonSchema []byte, initState []byte) (*JSONState, error) {
	rs := &jsonschema.Schema{}
	err := json.Unmarshal(jsonSchema, rs)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %s", err)
	}
	keyErrs, err := rs.ValidateBytes(context.Background(), initState)
	if err != nil {
		return nil, fmt.Errorf("error validating initial state: %s", err)
	}
	if len(keyErrs) != 0 {
		return nil, keyError(keyErrs)
	}

	return &JSONState{
		schema: rs,
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
	keyErrs, err := s.schema.ValidateBytes(context.Background(), updated)
	if err != nil {
		return nil, fmt.Errorf("error validating updated state: %s", err)
	}
	if len(keyErrs) != 0 {
		return nil, keyError(keyErrs)
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
	schema, err := json.Marshal(s.schema)
	if err != nil {
		return nil, err
	}
	return NewJSONState(schema, s.state)
}
