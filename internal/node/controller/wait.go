package controller

import (
	"fmt"
	"time"

	"github.com/eagraf/habitat-new/internal/node/hdb"
)

const TimeoutSeconds = 30 * time.Second

type WaiterPredicate func(hdb.State) (bool, error)

type WaiterSubscriber struct {
	name      string
	predicate WaiterPredicate

	doneChan chan hdb.StateUpdate
	errChan  chan error
}

func NewWaiterSubscriber(name string, predicate WaiterPredicate) *WaiterSubscriber {
	return &WaiterSubscriber{
		name:      name,
		predicate: predicate,
		doneChan:  make(chan hdb.StateUpdate),
		errChan:   make(chan error),
	}
}

func (s *WaiterSubscriber) Name() string {
	return fmt.Sprintf("StateUpdateWaiter-%s", s.name)
}

func (s *WaiterSubscriber) ConsumeEvent(update hdb.StateUpdate) error {
	done, err := s.predicate(update.NewState())
	if err != nil {
		s.errChan <- err
		return nil
	}
	if done {
		s.doneChan <- update
	}
	return nil
}

// WaitForState waits for the state to satisfy the predicate.
// If the state already satisfies the predicate, it will return immediately.
// If the state never satisfies the predicate, it will return an error after the timeout.
func (c *BaseNodeController) WaitForState(predicate WaiterPredicate) error {
	subscriber := NewWaiterSubscriber("WaitForStateUpdate", predicate)

	// Check if we're already in a valid state
	nodeState, err := c.GetNodeState()
	if err != nil {
		return err
	}
	// Check current state against predicate
	done, err := predicate(nodeState)
	if err != nil {
		return err
	}
	// If predicate is already satisfied, return immediately
	if done {
		return nil
	}

	// Add the subscriber to the channel and make sure to remove it when we're done
	c.stateUpdatesChannel.AddSubscriber(subscriber)
	// Remove the subscriber when we're done
	defer c.stateUpdatesChannel.RemoveSubscriber(subscriber)

	// Set up a timeout channel
	timeoutChan := make(chan struct{})
	go func() {
		time.Sleep(TimeoutSeconds)
		timeoutChan <- struct{}{}
	}()

	select {
	case <-subscriber.doneChan:
		return nil
	case err := <-subscriber.errChan:
		return err
	case <-timeoutChan:
		return fmt.Errorf("timeout waiting for state update on waiter %s", subscriber.Name())
	}
}
