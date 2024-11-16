package fishtail

import (
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/rs/zerolog/log"

	types "github.com/eagraf/habitat-new/core/api"
)

type Ingester struct {
	// Queue of chains of records to ingest
	chainQueue chan *RecordChainIngester
	pdsClient  *controller.PDSClient
}

func NewIngester(pdsClient *controller.PDSClient) *Ingester {
	return &Ingester{
		chainQueue: make(chan *RecordChainIngester, 1000),
		pdsClient:  pdsClient,
	}
}

func (i *Ingester) EnqueueChain(chain *RecordChainIngester) {
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
		log.Info().Msgf("ingested initial record of collection %s", syntax.ATURI(chain.initialURI).Collection().String())

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

type RecordChainIngester struct {
	initialCBORRecord []byte
	initialCID        string
	initialURI        string

	toIngest  chan syntax.ATURI
	pdsClient *controller.PDSClient

	seen map[string]interface{}

	ingestedRecords []*types.PDSGetRecordResponse

	atProtoEventPublisher *ATProtoEventPublisher
}

type IngestedRecordChain struct {
	Collection       string                        `json:"collection"`
	InitialRecordURI string                        `json:"initial_record_uri"`
	Records          []*types.PDSGetRecordResponse `json:"records"`
}

func NewRecordChainIngestor(
	initialCBORRecord []byte,
	initialCID string,
	initialURI string,
	atProtoEventPublisher *ATProtoEventPublisher,
) *RecordChainIngester {
	return &RecordChainIngester{
		initialCBORRecord: initialCBORRecord,
		initialCID:        initialCID,
		initialURI:        initialURI,

		toIngest:              make(chan syntax.ATURI, 1000),
		seen:                  make(map[string]interface{}),
		atProtoEventPublisher: atProtoEventPublisher,
	}
}

func (ic *RecordChainIngester) EnqueueRecord(uri syntax.ATURI) bool {
	log.Info().Msgf("enqueueing record: %s", uri)
	if _, ok := ic.seen[uri.String()]; !ok {
		ic.seen[uri.String()] = true
		ic.toIngest <- uri
		return true
	}
	return false
}

// IngestNext ingests the next record in the chain. If there are no more records to ingest, it returns false.
func (ic *RecordChainIngester) IngestNext() (bool, error) {
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
func (ic *RecordChainIngester) StartIngestion() error {
	// The initial record we ingest is using CBOR because it comes from the event stream.

	linkedURIs := make([]syntax.ATURI, 0)
	extractAtProtoURIs := getVisitorFunc(&linkedURIs)

	err := traverseCBOR(ic.initialCBORRecord, extractAtProtoURIs)
	if err != nil {
		return fmt.Errorf("failed to traverse CBOR: %w", err)
	}
	log.Info().Msgf("found %d linked URIs", len(linkedURIs))

	for _, uri := range linkedURIs {
		added := ic.EnqueueRecord(uri)
		if !added {
			log.Warn().Msgf("skipping duplicate linked record: %s", uri)
		}
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

func (ic *RecordChainIngester) FinishIngestion() error {
	if ic.ingestedRecords == nil || len(ic.ingestedRecords) == 0 {
		return fmt.Errorf("no records to publish")
	}

	errs, err := ic.atProtoEventPublisher.Publish(&IngestedRecordChain{
		Collection:       syntax.ATURI(ic.initialURI).Collection().String(),
		InitialRecordURI: ic.initialURI,
		Records:          ic.ingestedRecords,
	})

	log.Info().Msgf("published ingestion chain with %d errors", len(errs))
	for _, err := range errs {
		log.Error().Err(err).Msg("failed to publish ingestion chain")
	}

	return err
}

func (ic *RecordChainIngester) ingestRecord(atURI syntax.ATURI) (*types.PDSGetRecordResponse, error) {
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
		added := ic.EnqueueRecord(uri)
		if !added {
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
