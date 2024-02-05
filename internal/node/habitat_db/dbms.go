package habitat_db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eagraf/habitat-new/internal/node/habitat_db/consensus"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/core"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const PersistenceDirectory string = "/var/lib/habitat_db/0.1"

type DatabaseManager struct {
	raft *consensus.ClusterService

	databases map[string]*Database

	publisher pubsub.Publisher[state.StateUpdate]
}

type Database struct {
	ID   string
	Name string

	state.StateMachineController
}

func (d *Database) StateDirectory() string {
	return filepath.Join(PersistenceDirectory, d.ID)
}

func (d *Database) DatabaseAddress() string {
	return fmt.Sprintf("http://localhost:7000/%s", d.ID)
}

func (d *Database) Protocol() string {
	return filepath.Join("/habitat-raft", "0.0.1", d.ID)
}

func NewDatabaseManager(publisher pubsub.Publisher[state.StateUpdate]) (*DatabaseManager, error) {
	// TODO this is obviously wrong
	host := "localhost"
	raft := consensus.NewClusterService(host)

	err := os.MkdirAll(PersistenceDirectory, 0600)
	if err != nil {
		return nil, err
	}

	dm := &DatabaseManager{
		publisher: publisher,
		databases: make(map[string]*Database),
		raft:      raft,
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
	dirs, err := os.ReadDir(PersistenceDirectory)
	if err != nil {
		return fmt.Errorf("Error reading existing databases : %s", err)
	}
	for _, dir := range dirs {
		dbID := dir.Name()
		log.Info().Msgf("Restoring database %s", dbID)
		dbDir := filepath.Join(PersistenceDirectory, dbID)

		typeBytes, err := os.ReadFile(filepath.Join(dbDir, "schema_type"))
		if err != nil {
			return fmt.Errorf("Error reading schema for database %s: %s", dbID, err)
		}
		schemaType := string(typeBytes)

		nameBytes, err := os.ReadFile(filepath.Join(dbDir, "name"))
		if err != nil {
			return fmt.Errorf("Error reading name for database %s: %s", dbID, err)
		}
		name := string(nameBytes)

		schema, err := schemas.GetSchema(schemaType)
		if err != nil {
			return err
		}

		fsm, err := state.NewRaftFSMAdapter(dbID, schema, nil)
		if err != nil {
			return fmt.Errorf("Error creating Raft adapter for database %s: %s", dbID, err)
		}

		cluster, err := dm.raft.RestoreNode(dbID, dbDir, fsm)
		if err != nil {
			return fmt.Errorf("Error restoring database %s: %s", dbID, err)
		}

		stateMachineController, err := schemas.StateMachineFactory(dbID, schemaType, nil, cluster, dm.publisher)
		if err != nil {
			return err
		}

		db := &Database{
			ID:                     dbID,
			Name:                   name,
			StateMachineController: stateMachineController,
		}

		dm.databases[dbID] = db
		db.StateMachineController.StartListening()
	}
	return nil
}

// CreateDatabase creates a new database with the given name and schema type.
// This is a no-op if a database with the same name already exists.
func (dm *DatabaseManager) CreateDatabase(name string, schemaType string, initState []byte) (core.Client, error) {
	// First ensure that no db has the same name
	err := dm.checkDatabaseExists(name)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()

	db := &Database{
		ID:   id,
		Name: name,
	}

	err = os.MkdirAll(db.StateDirectory(), 0600)
	if err != nil {
		return nil, err
	}

	schema, err := schemas.GetSchema(schemaType)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(filepath.Join(db.StateDirectory(), "schema_type"), []byte(schema.Name()), 0600)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(filepath.Join(db.StateDirectory(), "name"), []byte(db.Name), 0600)
	if err != nil {
		return nil, err
	}

	fsm, err := state.NewRaftFSMAdapter(db.ID, schema, nil)
	if err != nil {
		return nil, err
	}

	cluster, err := dm.raft.CreateCluster(id, db.StateDirectory(), fsm)
	if err != nil {
		return nil, err
	}

	stateMachineController, err := schemas.StateMachineFactory(db.ID, schemaType, nil, cluster, dm.publisher)
	if err != nil {
		return nil, err
	}
	db.StateMachineController = stateMachineController

	initTransition, err := schema.InitializationTransition(initState)
	if err != nil {
		return nil, err
	}

	db.StateMachineController.StartListening()

	_, err = db.StateMachineController.ProposeTransitions([]core.Transition{initTransition})
	if err != nil {
		return nil, err
	}

	dm.databases[id] = db

	return db, nil
}

func (dm *DatabaseManager) GetDatabaseClient(id string) (core.Client, error) {
	if db, ok := dm.databases[id]; ok {
		return db, nil
	} else {
		return nil, &DatabaseNotFoundError{databaseID: id}
	}
}

func (dm *DatabaseManager) GetDatabaseByName(name string) (core.Client, error) {
	for _, db := range dm.databases {
		if db.Name == name {
			return db, nil
		}
	}
	return nil, &DatabaseNotFoundError{databaseName: name}
}

func (dm *DatabaseManager) checkDatabaseExists(name string) error {
	// TODO we need a much cleaner way to do this. Maybe a db metadata file.
	dirs, err := os.ReadDir(PersistenceDirectory)
	if err != nil {
		return fmt.Errorf("Error reading existing databases : %s", err)
	}
	for _, dir := range dirs {
		nameFilePath := filepath.Join(PersistenceDirectory, dir.Name(), "name")
		dbName, err := os.ReadFile(nameFilePath)
		if err != nil {
			return fmt.Errorf("Error reading name for database %s: %s", dir.Name(), err)
		}

		if string(dbName) == name {
			return &DatabaseAlreadyExistsError{databaseName: name}
		}
	}
	return nil
}
