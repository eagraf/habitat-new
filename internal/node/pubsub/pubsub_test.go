package pubsub

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestEvent struct {
	contents string
}

type TestSubscriber struct {
	consumedEvents []*TestEvent
}

func (s *TestSubscriber) ConsumeEvent(e *TestEvent) error {
	if e == nil {
		return errors.New("No nil events allowed")
	}
	s.consumedEvents = append(s.consumedEvents, e)
	return nil
}

func TestSimplePublisher(t *testing.T) {
	subscriber1 := &TestSubscriber{
		consumedEvents: make([]*TestEvent, 0),
	}
	subscriber2 := &TestSubscriber{
		consumedEvents: make([]*TestEvent, 0),
	}

	sp := newSimplePublisher[TestEvent]()
	sp.AddSubscriber(subscriber1)
	sp.AddSubscriber(subscriber2)

	err := sp.PublishEvent(&TestEvent{contents: "test"})
	assert.Nil(t, err)

	assert.Equal(t, len(subscriber1.consumedEvents), 1)
	assert.Equal(t, len(subscriber2.consumedEvents), 1)

	err = sp.PublishEvent(nil)
	assert.NotNil(t, err)
}
