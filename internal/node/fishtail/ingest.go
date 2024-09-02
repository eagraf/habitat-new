package fishtail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/rs/zerolog/log"

	types "github.com/eagraf/habitat-new/core/api"
)

type Ingester struct {
	chainQueue chan *IngestionChain
	pdsClient  *controller.PDSClient
}

func NewIngester(pdsClient *controller.PDSClient) *Ingester {
	return &Ingester{
		chainQueue: make(chan *IngestionChain, 1000),
		pdsClient:  pdsClient,
	}
}

func (i *Ingester) EnqueueChain(chain *IngestionChain) {
	chain.pdsClient = i.pdsClient
	i.chainQueue <- chain
}

func (i *Ingester) Run() error {
	// Right now, we just ingest one chain at a time from the queue.
	for {
		chain := <-i.chainQueue
		log.Info().Msgf("got chain to ingest")

		// Ingest the initial record, which is a special case because it comes as CBOR
		// directly from the event stream.
		err := chain.StartIngestion()
		if err != nil {
			log.Error().Msgf("failed to ingest initial record: %s", err)
			continue
		}
		log.Info().Msgf("ingested initial record")

		cont := true
		for cont {
			shouldContinue, err := chain.IngestNext()
			if err != nil {
				log.Error().Msgf("failed to ingest next record: %s", err)
			}
			cont = shouldContinue
		}

		err = chain.FinishIngestion()
		if err != nil {
			log.Error().Msgf("failed to finish ingestion: %s", err)
		}
	}
}

type IngestionChain struct {
	initialCBORRecord []byte
	initialCID        string
	initialURI        string

	toIngest  chan syntax.ATURI
	pdsClient *controller.PDSClient

	seen map[string]interface{}

	ingestedRecords []*types.PDSGetRecordResponse
}

func NewIngestionChain(initialCBORRecord []byte, initialCID string, initialURI string) *IngestionChain {
	return &IngestionChain{
		initialCBORRecord: initialCBORRecord,
		initialCID:        initialCID,
		initialURI:        initialURI,

		toIngest: make(chan syntax.ATURI, 1000),
		seen:     make(map[string]interface{}),
	}
}

func (ic *IngestionChain) EnqueueRecord(uri syntax.ATURI) {
	ic.toIngest <- uri
}

// IngestNext ingests the next record in the chain. If there are no more records to ingest, it returns false.
func (ic *IngestionChain) IngestNext() (bool, error) {
	log.Info().Msgf("ingesting next record")

	// If there are no more records to ingest, return false
	if len(ic.toIngest) == 0 {
		return false, nil
	}

	// Get the next URI from the channel
	uri := <-ic.toIngest

	record, err := ic.ingestRecord(uri)
	if err != nil {
		return true, fmt.Errorf("failed to ingest record: %w", err)
	}

	ic.ingestedRecords = append(ic.ingestedRecords, record)

	return true, nil
}

// StartIngestion starts the ingestion chain with the given URI and initial CBOR record.
func (ic *IngestionChain) StartIngestion() error {
	// The initial record we ingest is using CBOR because it comes from the event stream.

	linkedURIs := make([]syntax.ATURI, 0)
	extractAtProtoURIs := getVisitorFunc(&linkedURIs)

	err := traverseCBOR(ic.initialCBORRecord, extractAtProtoURIs)
	if err != nil {
		return fmt.Errorf("failed to traverse CBOR: %w", err)
	}
	log.Info().Msgf("found %d linked URIs", len(linkedURIs))

	for _, uri := range linkedURIs {
		ic.EnqueueRecord(uri)
	}

	cborData, err := convertCBORToMapStringInterface(ic.initialCBORRecord)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	ic.ingestedRecords = append(ic.ingestedRecords, &types.PDSGetRecordResponse{
		URI:   ic.initialURI,
		CID:   ic.initialCID,
		Value: cborData,
	})

	return nil
}

func (ic *IngestionChain) FinishIngestion() error {
	// Send the ingested records to the pubsub system.
	log.Info().Msgf("finishing ingestion")

	// Temporary, just loop through each record and post directly to the ingest endpoint for pouch
	for _, record := range ic.ingestedRecords {
		recordJSON, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal record to JSON: %w", err)
		}
		resp, err := http.Post("https://habitat-dev.tail07d32.ts.net/pouch_api/api/v1/ingest", "application/json", bytes.NewBuffer(recordJSON))
		if err != nil {
			return fmt.Errorf("failed to send record to ingest endpoint: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("ingest endpoint returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
		}

		log.Info().Msgf("Successfully sent record to ingest endpoint")
	}
	return nil
}

func (ic *IngestionChain) ingestRecord(atURI syntax.ATURI) (*types.PDSGetRecordResponse, error) {
	log.Info().Msgf("ingesting linked record: %s", atURI)

	// Parse the ATURI
	recordResp, err := ic.pdsClient.GetRecord(
		atURI.Authority().String(),
		atURI.Collection().String(),
		atURI.RecordKey().String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch record from PDS: %w", err)
	}

	linkedURIs := make([]syntax.ATURI, 0)
	extractAtProtoURIs := getVisitorFunc(&linkedURIs)

	err = traverseJSON(recordResp.Value, extractAtProtoURIs)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse JSON: %w", err)
	}

	// Ingest linked records
	for _, uri := range linkedURIs {
		if _, ok := ic.seen[string(uri)]; !ok {
			ic.seen[string(uri)] = true
			ic.EnqueueRecord(uri)
		} else {
			log.Warn().Msgf("skipping duplicate linked record: %s", uri)
		}
	}

	return recordResp, nil
}

func getVisitorFunc(linkedURIs *[]syntax.ATURI) func(node interface{}) error {
	return func(node interface{}) error {
		switch val := node.(type) {
		case string:
			if strings.HasPrefix(val, "at://") {
				log.Info().Msgf("found linked URI: %s", val)
				*linkedURIs = append(*linkedURIs, syntax.ATURI(val))
			}
			return nil
		default:
			return nil
		}
	}
}
