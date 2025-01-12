package controller

import "github.com/eagraf/habitat-new/internal/node/hdb"

// Component
type Component[T any] interface {
	RestoreFromState(T) error
	// Potentially blocking
	Transition(transitionType hdb.Transition) (*chan struct{}, error)
	// Supported types
	SupportedTransitionTypes() []hdb.TransitionType
}
