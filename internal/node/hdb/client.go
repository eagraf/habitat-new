package hdb

type Client interface {
	DatabaseID() string
	ProposeTransitions(transitions []Transition) (*JSONState, error)
	ProposeTransitionsEnriched(transitions []Transition) (*JSONState, error)
	Bytes() []byte
}
