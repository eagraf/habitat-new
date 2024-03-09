package node

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/stretchr/testify/assert"
	"github.com/wI2L/jsondiff"
)

func TestMigrations(t *testing.T) {
	err := validateMigrations()
	assert.Nil(t, err)

	err = validateNodeSchemaMigrations()
	assert.Nil(t, err)
}

func TestDataMigrations(t *testing.T) {

	migrations, err := readSchemaMigrationFiles()
	if err != nil {
		t.Error(err)
	}

	initSchema, err := getSchemaVersion(migrations, "v0.0.1")
	if err != nil {
		t.Error(err)
	}

	initState, err := initSchema.InitState()
	if err != nil {
		t.Error(err)
	}

	diffPatch, err := NodeDataMigrations.GetMigrationPatch("v0.0.1", "v0.0.2", initState.(*NodeState))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(diffPatch))

	updated, err := applyPatchToState(diffPatch, initState.(*NodeState))
	assert.Nil(t, err)
	assert.Equal(t, "test", updated.TestField)

	err = validateStateForVersion("v0.0.2", updated)
	assert.Nil(t, err)

	// Sanity check that this fails
	err = validateStateForVersion("v0.0.3", updated)
	assert.NotNil(t, err)

	diffPatch2, err := NodeDataMigrations.GetMigrationPatch("v0.0.2", "v0.0.3", updated)
	assert.Nil(t, err)

	updated2, err := applyPatchToState(diffPatch2, updated)
	assert.Nil(t, err)
	assert.Equal(t, "", updated2.TestField)

	err = validateStateForVersion("v0.0.3", updated2)
	assert.Nil(t, err)
}

func applyPatchToState(diffPatch jsondiff.Patch, state *NodeState) (*NodeState, error) {
	stateBytes, err := state.Bytes()
	if err != nil {
		return nil, err
	}

	patchBytes, err := json.Marshal(diffPatch)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, err
	}

	updated, err := patch.Apply(stateBytes)
	if err != nil {
		return nil, err
	}

	var updatedNodeState *NodeState
	err = json.Unmarshal(updated, &updatedNodeState)
	if err != nil {
		return nil, err
	}

	return updatedNodeState, nil
}

func validateStateForVersion(version string, state *NodeState) error {
	migrations, err := readSchemaMigrationFiles()
	if err != nil {
		return err
	}
	schema, err := getSchemaVersion(migrations, version)
	if err != nil {
		return err
	}

	stateBytes, err := state.Bytes()
	if err != nil {
		return err
	}
	return schema.ValidateState(stateBytes)
}
