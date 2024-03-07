package node

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/wI2L/jsondiff"
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
	Up   []JSONPatch `json:"up"`
	Down []JSONPatch `json:"down"`

	upBytes   []byte
	downBytes []byte
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
