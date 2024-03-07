package migrations

import "github.com/eagraf/habitat-new/core/state/node"

type DataMigration interface {
	Version() string
	Up(*node.NodeState) (*node.NodeState, error)
	Down(*node.NodeState) (*node.NodeState, error)
}

// basicDataMigration is a simple implementation of DataMigration with some basic helpers.
type basicDataMigration struct {
	upVersion   string
	downVersion string
	up          func(*node.NodeState) (*node.NodeState, error)
	down        func(*node.NodeState) (*node.NodeState, error)
}

func (m *basicDataMigration) Version() string {
	return m.upVersion
}

func (m *basicDataMigration) Up(state *node.NodeState) (*node.NodeState, error) {
	return m.up(state)
}

func (m *basicDataMigration) Down(state *node.NodeState) (*node.NodeState, error) {
	return m.down(state)
}

// Rules for migrations:
// - Going upwards, fields can only be added not removed. New fields must be optional
// - Going downwards, fields can only be removed not added. Removed fields must be optional
var DataMigrations = []DataMigration{
	&basicDataMigration{
		upVersion:   "0.0.1",
		downVersion: "0.0.0",
		up: func(state *node.NodeState) (*node.NodeState, error) {
			return state, nil
		},
		down: func(state *node.NodeState) (*node.NodeState, error) {
			// The first down migration can never be run
			return nil, nil
		},
	},
}
