package state

import "reflect"

type Schema interface {
	Name() string
	InitState() (State, error)
	Bytes() []byte
	Type() reflect.Type
	InitializationTransition(initState []byte) (Transition, error)
}
