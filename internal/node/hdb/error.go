package hdb

type DatabaseNotFoundError struct {
	DatabaseName string
	DatabaseID   string
}

func (e *DatabaseNotFoundError) Error() string {
	if e.DatabaseName != "" {
		return "Database with name " + e.DatabaseName + " not found"
	} else {
		return "Database with id " + e.DatabaseID + " not found"
	}
}

type DatabaseAlreadyExistsError struct {
	DatabaseName string
}

func (e *DatabaseAlreadyExistsError) Error() string {
	return "Database with name " + e.DatabaseName + " already exists"
}
