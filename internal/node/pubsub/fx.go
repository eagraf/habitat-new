package pubsub

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

func NewSimplePublisher[E Event]() *SimplePublisher[E] {
	publisher := newSimplePublisher[E]()

	return publisher
}

func NewSimpleChannel[E Event](publishers []Publisher[E], subscribers []Subscriber[E], lc fx.Lifecycle) *SimpleChannel[E] {
	channel := newSimpleChannel[E]()
	channel.publishers = publishers
	channel.subscribers = subscribers
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				err := channel.Listen()
				if err != nil {
					log.Fatal().Err(err).Msgf("unrecoverable error listening to channel")
				}
			}()
			return nil
		},
	})

	return channel
}
