package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrations(t *testing.T) {
	err := validateMigrations()
	assert.Nil(t, err)

	err = validateNodeSchemaMigrations()
	assert.Nil(t, err)
}

func TestDataMigrations(t *testing.T) {

	nodeSchema := &NodeSchema{}
	initState, err := nodeSchema.InitState()
	if err != nil {
		t.Error(err)
	}

	diffPatch, err := NodeDataMigrations.GetMigrationPatch("v0.0.1", "v0.0.2", initState.(*NodeState))
	assert.Nil(t, err)
	assert.Equal(t, 2, len(diffPatch))

	updated, err := applyPatchToState(diffPatch, initState.(*NodeState))
	assert.Nil(t, err)
	assert.Equal(t, "test", updated.TestField)
	assert.Equal(t, "v0.0.2", updated.SchemaVersion)

	err = updated.Validate()
	assert.Nil(t, err)

	diffPatch2, err := NodeDataMigrations.GetMigrationPatch("v0.0.2", "v0.0.3", updated)
	assert.Nil(t, err)

	updated2, err := applyPatchToState(diffPatch2, updated)
	assert.Nil(t, err)
	assert.Equal(t, "", updated2.TestField)
	assert.Equal(t, "v0.0.3", updated2.SchemaVersion)

	err = updated2.Validate()
	assert.Nil(t, err)
}
