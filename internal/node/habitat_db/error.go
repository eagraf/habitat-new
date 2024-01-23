package habitat_db

type DatabaseNotFoundError struct {
	databaseName string
	databaseID   string
}

func (e *DatabaseNotFoundError) Error() string {
	if e.databaseName != "" {
		return "Database with name " + e.databaseName + " not found"
	} else {
		return "Database with id " + e.databaseID + " not found"
	}
}

type DatabaseAlreadyExistsError struct {
	databaseName string
}

func (e *DatabaseAlreadyExistsError) Error() string {
	return "Database with name " + e.databaseName + " already exists"
}
