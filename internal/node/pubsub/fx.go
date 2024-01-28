package pubsub

func NewSimplePublisher[E Event](subscriber []Subscriber[E]) *SimplePublisher[E] {
	publisher := newSimplePublisher[E]()
	for _, s := range subscriber {
		publisher.AddSubscriber(s)
	}

	return publisher
}
