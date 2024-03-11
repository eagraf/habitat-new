package node

import (
	"encoding/json"
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

	// Test migrating downwards
	diffPatch3, err := NodeDataMigrations.GetMigrationPatch("v0.0.3", "v0.0.2", updated2)
	assert.Nil(t, err)

	updated3, err := applyPatchToState(diffPatch3, updated2)
	assert.Nil(t, err)
	assert.Equal(t, "test", updated3.TestField)
	assert.Equal(t, "v0.0.2", updated3.SchemaVersion)
}

func TestBackwardsCompatibility(t *testing.T) {
	// Migrate the data all the way up, and then back down. This test is to ensure backwards
	// compatibility over long periods. It doesn't insert new data in migrations in the middle,
	// so it is mostly testing that data from very early versions will travel up and down correctly.
	// Specific important migrations should have their own tests.
	//
	// IMPORTANT: Any code that breaks this test breaks backwards compatibility. Think hard before
	// changing this test to make your code pass.

	// NodeSchema in v0.0.1
	nodeState := &NodeState{
		NodeID:        "node1",
		Name:          "My Node",
		Certificate:   "Fake certificate",
		SchemaVersion: "v0.0.1",
		Users: map[string]*User{
			"user1": {
				ID:          "user1",
				Username:    "username1",
				Certificate: "fake user certificate",
				AppInstallations: []string{
					"app1",
					"app2",
				},
			},
		},
		AppInstallations: map[string]*AppInstallationState{
			"app1": {
				AppInstallation: &AppInstallation{
					ID:      "app1",
					Name:    "appname1",
					Version: "1.0.0",
					Package: Package{
						Driver:             "test",
						RegistryURLBase:    "https://registry.example.com",
						RegistryPackageID:  "appname1",
						RegistryPackageTag: "1.0.0",
					},
				},
			},
			"app2": {
				AppInstallation: &AppInstallation{
					ID:      "app2",
					Name:    "appname2",
					Version: "1.0.0",
					Package: Package{
						Driver:             "test",
						RegistryURLBase:    "https://registry.example.com",
						RegistryPackageID:  "appname1",
						RegistryPackageTag: "1.0.0",
					},
				},
			},
		},
		Processes: map[string]*ProcessState{
			"proc1": {
				Process: &Process{
					ID:      "proc1",
					AppID:   "app1",
					UserID:  "user1",
					Created: "now",
					Driver:  "test",
				},
				State: ProcessStateRunning,
			},
			// This process was not in a running state, but should be started
			"proc2": {
				Process: &Process{
					ID:      "proc2",
					AppID:   "app2",
					UserID:  "user1",
					Created: "now",
					Driver:  "test",
				},
				State: ProcessStateStarting,
			},
		},
	}

	diffPatch, err := NodeDataMigrations.GetMigrationPatch("v0.0.1", LatestVersion, nodeState)
	assert.Nil(t, err)

	updated, err := applyPatchToState(diffPatch, nodeState)
	assert.Nil(t, err)

	// Validate against the latest schema
	err = updated.Validate()
	assert.Nil(t, err)

	// Migrate back down
	downPatch, err := NodeDataMigrations.GetMigrationPatch(LatestVersion, "v0.0.1", updated)
	assert.Nil(t, err)

	updatedDown, err := applyPatchToState(downPatch, updated)
	assert.Nil(t, err)

	// Assert the serialized version of the beginning and end state are the same
	// Pretty print them so that the diff is easilly viewable in test output
	serialized, err := json.MarshalIndent(nodeState, "", "  ")
	assert.Nil(t, err)

	serializedDown, err := json.MarshalIndent(updatedDown, "", "  ")
	assert.Nil(t, err)

	assert.Equal(t, string(serialized), string(serializedDown))
}
