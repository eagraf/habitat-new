package pubsub

type Event interface {
}

type Publisher[E Event] interface {
	PublishEvent(*E) error
	AddSubscriber(Subscriber[E])
}

type Subscriber[E Event] interface {
	ConsumeEvent(*E) error
}

// SimplePublisher is an extremely simple implementation of a Publisher that just
// loops through each subscriber and calls ConsumeEvent on it. It is not thread safe,
// and does not ensure the ordering of events in any way. In the future, a more mature
// solution that guarantees these properties is likely needed.
// TODO: Implement a channel based publisher.
type SimplePublisher[E Event] struct {
	subscribers []Subscriber[E]
}

func newSimplePublisher[E Event]() *SimplePublisher[E] {
	return &SimplePublisher[E]{
		subscribers: make([]Subscriber[E], 0),
	}
}

func (p *SimplePublisher[E]) PublishEvent(e *E) error {
	for _, s := range p.subscribers {
		err := s.ConsumeEvent(e)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *SimplePublisher[E]) AddSubscriber(s Subscriber[E]) {
	p.subscribers = append(p.subscribers, s)
}
