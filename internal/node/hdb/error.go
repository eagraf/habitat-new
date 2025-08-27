package hdb

import "errors"

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

var (
	DatabaseAlreadyExistsError = errors.New("database already exists")
)
