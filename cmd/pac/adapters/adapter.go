package adapters

import (
	"errors"
)

// ErrNotInstalled is returned when a tool is not installed
var ErrNotInstalled = errors.New("tool is not installed")

// Adapter defines the interface for 3rd party CLI adapters
type Adapter interface {
	// GetVersion returns the version of the installed tool
	// Returns ErrNotInstalled if the tool is not available
	GetVersion() (string, error)

	// GetInstallInstructions returns instructions for installing the tool
	GetInstallInstructions() string
}
