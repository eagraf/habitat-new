package hdb

type Client interface {
	ProposeTransitions(transitions []Transition) (*JSONState, error)
	Bytes() []byte
}
