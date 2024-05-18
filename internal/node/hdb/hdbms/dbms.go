package hdbms

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/consensus"
	"github.com/eagraf/habitat-new/internal/node/hdb/state"
	"github.com/eagraf/habitat-new/internal/node/hdb/state/schemas"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type DatabaseManager struct {
	nodeConfig *config.NodeConfig
	raft       *consensus.ClusterService

	databases map[string]*Database

	publisher pubsub.Publisher[hdb.StateUpdate]
}

type Database struct {
	id   string
	name string
	path string

	state.StateMachineController
}

func (d *Database) ID() string {
	return d.id
}

func (d *Database) Name() string {
	return d.name
}

func (d *Database) Path() string {
	return d.path
}

func (d *Database) DatabaseAddress() string {
	return fmt.Sprintf("http://localhost:7000/%s", d.id)
}

func (d *Database) Protocol() string {
	return filepath.Join("/habitat-raft", "0.0.1", d.id)
}

func NewDatabaseManager(config *config.NodeConfig, publisher pubsub.Publisher[hdb.StateUpdate]) (*DatabaseManager, error) {
	// TODO this is obviously wrong
	host := "localhost"
	raft := consensus.NewClusterService(host)

	err := os.MkdirAll(config.HDBPath(), 0700)
	if err != nil {
		return nil, err
	}

	dm := &DatabaseManager{
		nodeConfig: config,
		publisher:  publisher,
		databases:  make(map[string]*Database),
		raft:       raft,
	}

	return dm, nil
}

func (dm *DatabaseManager) Start() {
	for _, db := range dm.databases {
		db.StateMachineController.StartListening()
	}
}

func (dm *DatabaseManager) Stop() {
	for _, db := range dm.databases {
		db.StateMachineController.StopListening()
	}
}

func (dm *DatabaseManager) RestartDBs() error {
	dirs, err := os.ReadDir(dm.nodeConfig.HDBPath())
	if err != nil {
		return fmt.Errorf("error reading existing databases : %s", err)
	}
	for _, dir := range dirs {
		dbID := dir.Name()
		log.Info().Msgf("Restoring database %s", dbID)
		dbDir := filepath.Join(dm.nodeConfig.HDBPath(), dbID)

		typeBytes, err := os.ReadFile(filepath.Join(dbDir, "schema_type"))
		if err != nil {
			return fmt.Errorf("error reading schema for database %s: %s", dbID, err)
		}
		schemaType := string(typeBytes)

		nameBytes, err := os.ReadFile(filepath.Join(dbDir, "name"))
		if err != nil {
			return fmt.Errorf("error reading name for database %s: %s", dbID, err)
		}
		name := string(nameBytes)

		schema, err := schemas.GetSchema(schemaType)
		if err != nil {
			return err
		}

		fsm, err := state.NewRaftFSMAdapter(dbID, schema, nil)
		if err != nil {
			return fmt.Errorf("error creating Raft adapter for database %s: %s", dbID, err)
		}

		db := &Database{
			id:   dbID,
			name: name,
			path: filepath.Join(dm.nodeConfig.HabitatPath(), "hdb", dbID),
		}

		cluster, err := dm.raft.RestoreNode(db, fsm)
		if err != nil {
			return fmt.Errorf("error restoring database %s: %s", dbID, err)
		}

		stateMachineController, err := schemas.StateMachineFactory(dbID, schemaType, nil, cluster, dm.publisher)
		if err != nil {
			return err
		}

		db.StateMachineController = stateMachineController

		dm.databases[dbID] = db
		db.StateMachineController.StartListening()
	}
	return nil
}

// CreateDatabase creates a new database with the given name and schema type.
// This is a no-op if a database with the same name already exists.
func (dm *DatabaseManager) CreateDatabase(name string, schemaType string, initState []byte) (hdb.Client, error) {
	// First ensure that no db has the same name
	err := dm.checkDatabaseExists(name)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()

	db := &Database{
		id:   id,
		name: name,
		path: filepath.Join(dm.nodeConfig.HabitatPath(), "hdb", id),
	}

	err = os.MkdirAll(db.Path(), 0700)
	if err != nil {
		return nil, err
	}

	schema, err := schemas.GetSchema(schemaType)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(filepath.Join(db.Path(), "schema_type"), []byte(schema.Name()), 0600)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(filepath.Join(db.Path(), "name"), []byte(db.name), 0600)
	if err != nil {
		return nil, err
	}

	fsm, err := state.NewRaftFSMAdapter(db.id, schema, nil)
	if err != nil {
		return nil, err
	}

	cluster, err := dm.raft.CreateCluster(db, fsm)
	if err != nil {
		return nil, err
	}

	stateMachineController, err := schemas.StateMachineFactory(db.id, schemaType, nil, cluster, dm.publisher)
	if err != nil {
		return nil, err
	}
	db.StateMachineController = stateMachineController

	initTransition, err := schema.InitializationTransition(initState)
	if err != nil {
		return nil, err
	}

	db.StateMachineController.StartListening()

	_, err = db.StateMachineController.ProposeTransitions([]hdb.Transition{initTransition})
	if err != nil {
		return nil, err
	}

	dm.databases[id] = db

	return db, nil
}

func (dm *DatabaseManager) GetDatabaseClient(id string) (hdb.Client, error) {
	if db, ok := dm.databases[id]; ok {
		return db, nil
	} else {
		return nil, &hdb.DatabaseNotFoundError{DatabaseID: id}
	}
}

func (dm *DatabaseManager) GetDatabaseClientByName(name string) (hdb.Client, error) {
	for _, db := range dm.databases {
		if db.name == name {
			return db, nil
		}
	}
	return nil, &hdb.DatabaseNotFoundError{DatabaseName: name}
}

func (dm *DatabaseManager) checkDatabaseExists(name string) error {
	// TODO we need a much cleaner way to do this. Maybe a db metadata file.
	dirs, err := os.ReadDir(dm.nodeConfig.HDBPath())
	if err != nil {
		return fmt.Errorf("error reading existing databases : %s", err)
	}
	for _, dir := range dirs {
		nameFilePath := filepath.Join(dm.nodeConfig.HDBPath(), dir.Name(), "name")
		dbName, err := os.ReadFile(nameFilePath)
		if err != nil {
			return fmt.Errorf("error reading name for database %s: %s", dir.Name(), err)
		}

		if string(dbName) == name {
			return &hdb.DatabaseAlreadyExistsError{DatabaseName: name}
		}
	}
	return nil
}
