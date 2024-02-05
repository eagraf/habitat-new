package hdb

type HDBManager interface {
	Start()
	Stop()
	RestartDBs() error
	CreateDatabase(name, schemaType string, initState []byte) (Client, error)
	GetDatabaseClient(id string) (Client, error)
	GetDatabaseClientByName(name string) (Client, error)
}
