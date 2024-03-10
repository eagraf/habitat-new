package node

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/wI2L/jsondiff"
	"golang.org/x/mod/semver"
)

// TODO factor this out to be reusable across schemas

//go:embed migrations/*
var migrationsDir embed.FS

type JSONPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

type Migration struct {
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`

	Up   []JSONPatch `json:"up"`
	Down []JSONPatch `json:"down"`

	upBytes   []byte
	downBytes []byte
}

func getSchemaVersion(migrations []*Migration, targetVersion string) ([]byte, error) {
	// TODO validate up to a certain point. Right now this just validates
	// all migrations up to the full schema, but we might want to stop at an
	// intermediate state.

	curSchema := "{}"
	for _, mig := range migrations {

		patch, err := jsonpatch.DecodePatch(mig.upBytes)
		if err != nil {
			return nil, err
		}

		updated, err := patch.Apply([]byte(curSchema))
		if err != nil {
			return nil, err
		}

		curSchema = string(updated)

		if mig.NewVersion == targetVersion {
			break
		} else if semver.Compare(mig.NewVersion, targetVersion) > 0 {
			return nil, fmt.Errorf("target version %s not found in migrations. latest version found: %s", targetVersion, mig.NewVersion)
		}
	}

	return []byte(curSchema), nil
}

func validateMigrations() error {
	// TODO validate up to a certain point. Right now this just validates
	// all migrations up to the full schema, but we might want to stop at an
	// intermediate state.

	migrationsFiles, err := migrationsDir.ReadDir("migrations")
	if err != nil {
		return err
	}

	// Read in all the migration data
	migrations := make([]*Migration, 0)
	for _, migFile := range migrationsFiles {
		if migFile.IsDir() {
			continue
		}
		if !strings.HasSuffix(migFile.Name(), ".json") {
			continue
		}

		migFileData, err := migrationsDir.ReadFile("migrations/" + migFile.Name())
		if err != nil {
			return err
		}

		var migration Migration
		err = json.Unmarshal(migFileData, &migration)
		if err != nil {
			return err
		}

		// Convert the JSONPatch arrays to bytes
		migration.upBytes, err = json.Marshal(migration.Up)
		if err != nil {
			return err
		}
		migration.downBytes, err = json.Marshal(migration.Down)
		if err != nil {
			return err
		}

		migrations = append(migrations, &migration)
	}

	curSchema := "{}"

	// Test going up
	for _, mig := range migrations {

		patch, err := jsonpatch.DecodePatch(mig.upBytes)
		if err != nil {
			return err
		}

		updated, err := patch.Apply([]byte(curSchema))
		if err != nil {
			return err
		}

		curSchema = string(updated)
	}

	err = compareSchemas(nodeSchemaRaw, curSchema)
	if err != nil {
		return err
	}

	// Test going down
	for i := len(migrations) - 1; i >= 0; i-- {
		mig := migrations[i]

		patch, err := jsonpatch.DecodePatch(mig.downBytes)
		if err != nil {
			return err
		}

		updated, err := patch.Apply([]byte(curSchema))
		if err != nil {
			return err
		}

		curSchema = string(updated)
	}

	if curSchema != "{}" {
		return fmt.Errorf("down migrations do not result in {}")
	}

	return nil
}

func validateNodeSchemaMigrations() error {
	migrations, err := readSchemaMigrationFiles()
	if err != nil {
		return err
	}

	schema, err := getSchemaVersion(migrations, LatestVersion)
	if err != nil {
		return err
	}

	err = compareSchemas(nodeSchemaRaw, string(schema))
	if err != nil {
		return fmt.Errorf("latest schema: %s\nderived schema: %s\ndiff: %s", nodeSchemaRaw, string(schema), err)
	}

	return nil

}

func readSchemaMigrationFiles() ([]*Migration, error) {
	migrationsFiles, err := migrationsDir.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	// Read in all the migration data
	migrations := make([]*Migration, 0)
	for _, migFile := range migrationsFiles {
		if migFile.IsDir() {
			continue
		}
		if !strings.HasSuffix(migFile.Name(), ".json") {
			continue
		}

		migFileData, err := migrationsDir.ReadFile("migrations/" + migFile.Name())
		if err != nil {
			return nil, err
		}

		var migration Migration
		err = json.Unmarshal(migFileData, &migration)
		if err != nil {
			return nil, err
		}

		// Convert the JSONPatch arrays to bytes
		migration.upBytes, err = json.Marshal(migration.Up)
		if err != nil {
			return nil, err
		}
		migration.downBytes, err = json.Marshal(migration.Down)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, &migration)
	}

	return migrations, nil
}

func compareSchemas(expected, actual string) error {
	var expectedMap, actualMap map[string]interface{}
	err := json.Unmarshal([]byte(expected), &expectedMap)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(actual), &actualMap); err != nil {
		return err
	}

	patch, err := jsondiff.Compare(expectedMap, actualMap)
	if err != nil {
		return err
	}

	if len(patch) > 0 {
		return fmt.Errorf("schemas are not equal: %s", patch)
	}

	return nil
}

type MigrationsList []DataMigration

func (m MigrationsList) GetNeededMigrations(currentVersion, targetVersion string) ([]DataMigration, error) {
	migrations := make([]DataMigration, 0)
	inMigration := false
	for _, dataMigration := range NodeDataMigrations {
		if dataMigration.UpVersion() == currentVersion {
			inMigration = true
			continue
		} else if inMigration {
			migrations = append(migrations, dataMigration)
		}
		if dataMigration.UpVersion() == targetVersion {
			inMigration = false
			break
		}
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}
	if inMigration {
		return nil, fmt.Errorf("couldn't migrate up to target version, latest was %s", migrations[len(migrations)-1].UpVersion())
	}
	return migrations, nil
}

func (m MigrationsList) GetMigrationPatch(currentVersion, targetVersion string, startState *NodeState) (jsondiff.Patch, error) {
	migrations, err := m.GetNeededMigrations(currentVersion, targetVersion)
	if err != nil {
		return nil, err
	}
	curState, err := startState.Copy()
	if err != nil {
		return nil, err
	}
	for _, dataMigration := range migrations {
		// Run the up migration
		curState, err = dataMigration.Up(curState)
		if err != nil {
			return nil, err
		}
		if dataMigration.UpVersion() == targetVersion {
			break
		}
	}

	curState.SchemaVersion = targetVersion

	patch, err := jsondiff.Compare(startState, curState)
	if err != nil {
		return nil, err
	}

	return patch, nil
}

type DataMigration interface {
	UpVersion() string
	DownVersion() string
	Up(*NodeState) (*NodeState, error)
	Down(*NodeState) (*NodeState, error)
}

// basicDataMigration is a simple implementation of DataMigration with some basic helpers.
type basicDataMigration struct {
	upVersion   string
	downVersion string

	// Functions for moving data up and down a version
	up   func(*NodeState) (*NodeState, error)
	down func(*NodeState) (*NodeState, error)
}

func (m *basicDataMigration) UpVersion() string {
	return m.upVersion
}

func (m *basicDataMigration) DownVersion() string {
	return m.downVersion
}

func (m *basicDataMigration) Up(state *NodeState) (*NodeState, error) {
	return m.up(state)
}

func (m *basicDataMigration) Down(state *NodeState) (*NodeState, error) {
	return m.down(state)
}

// Rules for migrations:
// - Going upwards, fields can only be added not removed. New fields must be optional
// - Going downwards, fields can only be removed not added. Removed fields must be optional
var NodeDataMigrations = MigrationsList{
	&basicDataMigration{
		upVersion:   "v0.0.1",
		downVersion: "v0.0.0",
		up: func(state *NodeState) (*NodeState, error) {
			return state, nil
		},
		down: func(state *NodeState) (*NodeState, error) {
			// The first down migration can never be run
			return nil, nil
		},
	},
	&basicDataMigration{
		upVersion:   "v0.0.2",
		downVersion: "v0.0.1",
		up: func(state *NodeState) (*NodeState, error) {
			newState, err := state.Copy()
			if err != nil {
				return nil, err
			}
			newState.TestField = "test"
			return newState, nil
		},
		down: func(state *NodeState) (*NodeState, error) {
			newState, err := state.Copy()
			if err != nil {
				return nil, err
			}
			newState.TestField = ""
			return newState, nil
		},
	},
	&basicDataMigration{
		upVersion:   "v0.0.3",
		downVersion: "v0.0.2",
		up: func(state *NodeState) (*NodeState, error) {
			newState, err := state.Copy()
			if err != nil {
				return nil, err
			}
			newState.TestField = ""
			return newState, nil
		},
		down: func(state *NodeState) (*NodeState, error) {
			newState, err := state.Copy()
			if err != nil {
				return nil, err
			}
			newState.TestField = "test"
			return newState, nil
		},
	},
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
