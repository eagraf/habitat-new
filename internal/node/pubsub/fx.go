package pubsub

import (
	"github.com/rs/zerolog/log"
)

func NewSimplePublisher[E Event]() *SimplePublisher[E] {
	publisher := newSimplePublisher[E]()

	return publisher
}

func NewSimpleChannel[E Event](publishers []Publisher[E], subscribers []Subscriber[E]) *SimpleChannel[E] {
	channel := newSimpleChannel[E]()
	channel.publishers = publishers
	channel.subscribers = subscribers

	go func() {
		err := channel.Listen()
		if err != nil {
			log.Fatal().Err(err).Msgf("unrecoverable error listening to channel")
		}
	}()
	return channel
}
