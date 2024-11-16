package fishtail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/rs/zerolog/log"
)

type ATProtoEventPublisher struct {
	nodeConfig    *config.NodeConfig
	subscriptions map[string][]string // maps atproto collection names to subscribed application endpoints
	mu            sync.RWMutex
	httpClient    *http.Client
}

func NewATProtoEventPublisher(nodeConfig *config.NodeConfig) *ATProtoEventPublisher {
	return &ATProtoEventPublisher{
		nodeConfig:    nodeConfig,
		subscriptions: make(map[string][]string),
		httpClient:    &http.Client{},
	}
}

func (sc *ATProtoEventPublisher) AddSubscription(lexicon, target string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if _, exists := sc.subscriptions[lexicon]; !exists {
		sc.subscriptions[lexicon] = []string{}
	}
	sc.subscriptions[lexicon] = append(sc.subscriptions[lexicon], target)
}

func (sc *ATProtoEventPublisher) GetSubscriptions(lexicon string) []string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	return sc.subscriptions[lexicon]
}

// Publish sends an ingestion chain to all of the subscribers. This function returns a list of errors
// for each subscriber that failed to receive the ingestion chain, and an overall error for unexpected
// errors encountered while publishing the ingestion chain.
func (sc *ATProtoEventPublisher) Publish(ingestionChain *IngestedRecordChain) ([]error, error) {
	// When a certain record is modified, we want to publish it to the subscribers.
	// In addition to that record, we traverse the chain of all the linked records
	// and send them in the same message. That way, the various subscribers will
	// have up-to-date info without the need to query the same data from thhe PDS.

	// Find subscribers based on the collection of the initial record
	subscribers := sc.GetSubscriptions(ingestionChain.Collection)

	if len(subscribers) == 0 {
		// No subscribers for this collection, nothing to do
		return nil, nil
	}

	body, err := json.Marshal(ingestionChain)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ingestion chain: %w", err)
	}

	// Track errors
	errors := make([]error, 0)

	// For now, we'll just log the action
	for _, target := range subscribers {
		// TODO: these http requests should be batched and queued.

		// Send a single POST request with all records
		resp, err := sc.httpClient.Post(target, "application/json", bytes.NewBuffer(body))
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to send to %s: %w", target, err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errors = append(errors, fmt.Errorf("endpoint %s returned status %d: %s", target, resp.StatusCode, string(body)))
			continue
		}

		log.Info().Msgf("Successfully sent %d records to ingest endpoint", len(ingestionChain.Records))
	}

	return errors, nil
}
