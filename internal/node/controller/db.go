package controller

import (
	"encoding/json"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"golang.org/x/mod/semver"
)

func MigrateNodeDB(db hdb.Client, targetVersion string) error {
	var nodeState node.State
	err := json.Unmarshal(db.Bytes(), &nodeState)
	if err != nil {
		return nil
	}

	// No-op if version is already the target
	if semver.Compare(nodeState.SchemaVersion, targetVersion) == 0 {
		return nil
	}

	_, err = db.ProposeTransitions([]hdb.Transition{
		node.CreateMigrationTransition(targetVersion),
	})
	return err
}
