package fishtail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/rs/zerolog/log"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repomgr"

	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/repo"
	"github.com/gorilla/websocket"
)

type TailService interface {
}

// Fishtail is a service that listens to events from the Habitat node's PDS, ingesting them
// and other linked records, and then publishes the relevant records to subscribing apps /ingest
// endpoints, so that they can each index the data as needed.
type Fishtail struct {
	relayHost string

	ingester *Ingester

	atprotoEventPublisher *ATProtoEventPublisher
}

func NewFishtailService(relayHost string, nodeConfig *config.NodeConfig, atProtoEventPublisher *ATProtoEventPublisher) *Fishtail {
	return &Fishtail{
		relayHost:             relayHost,
		ingester:              NewIngester(controller.NewPDSClient(nodeConfig)),
		atprotoEventPublisher: atProtoEventPublisher,
	}
}

// FirehoseConsumer starts a goroutine dedicated to consuming the PDS event stream firehose over
// a websocket connection. Each new commit event starts an ingestion chain, which is processed by
// the LinkedRecordIngester.
func (f *Fishtail) FirehoseConsumer(ctx context.Context, relayHost string) func() error {
	return func() error {
		return f.runFirehoseConsumer(
			ctx,
			relayHost,
		)
	}
}

// LinkedRecordIngester starts a goroutine dedicated to processing ingestion chains as they are
// received from the FirehoseConsumer. Each chain starts with the initial record that was specified
// in the firehose event, and then adds any linked records to the chain as they are discovered.
func (f *Fishtail) LinkedRecordIngester() func() error {
	return func() error {
		return f.ingester.Run()
	}
}

func (f *Fishtail) runFirehoseConsumer(ctx context.Context, relayHost string) error {
	log.Info().Msgf("subscribing to firehose: %v", relayHost)

	dialer := websocket.DefaultDialer
	u, err := url.Parse(relayHost)
	if err != nil {
		return fmt.Errorf("invalid relayHost URI: %w", err)
	}

	// always continue at the current cursor offset (don't provide cursor query param)
	u.Path = "xrpc/com.atproto.sync.subscribeRepos"
	log.Info().Msgf("subscribing to repo event stream: %v", relayHost)
	con, _, err := dialer.Dial(u.String(), http.Header{
		"User-Agent": []string{"fishtail/0.0.1"},
	})
	if err != nil {
		return fmt.Errorf("subscribing to firehose failed (dialing): %w", err)
	}

	rsc := &events.RepoStreamCallbacks{
		RepoCommit: func(evt *comatproto.SyncSubscribeRepos_Commit) error {
			return f.handleRepoCommit(ctx, evt)
		},
		// NOTE: could add other callbacks as needed
	}

	var scheduler events.Scheduler
	// use parallel scheduler
	parallelism := 4
	scheduler = parallel.NewScheduler(
		parallelism,
		1000,
		relayHost,
		rsc.EventHandler,
	)
	log.Info().Msgf("firehose scheduler configured: %v", scheduler)

	return events.HandleRepoStream(ctx, con, scheduler)
}

// NOTE: for now, this function basically never errors, just logs and returns nil. Should think through error processing better.
func (f *Fishtail) handleRepoCommit(ctx context.Context, evt *comatproto.SyncSubscribeRepos_Commit) error {
	log.Info().Msgf("received repo commit event: %+v", evt)
	log.Info().Msgf("event blocks (base64): %s", base64.StdEncoding.EncodeToString(evt.Blocks))
	eventJSON, err := json.Marshal(evt)
	if err != nil {
		log.Error().Msgf("failed to marshal event to JSON: %s", err)
	} else {
		log.Info().Msgf("full event JSON: %s", string(eventJSON))
	}

	if evt.TooBig {
		log.Warn().Msg("skipping tooBig events for now")
		return nil
	}

	rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
	if err != nil {
		log.Error().Msgf("failed to read repo from car %s", err)
		return nil
	}

	for _, op := range evt.Ops {
		ek := repomgr.EventKind(op.Action)
		switch ek {
		case repomgr.EvtKindCreateRecord, repomgr.EvtKindUpdateRecord:
			// read the record bytes from blocks, and verify CID
			rc, recordCBOR, err := rr.GetRecordBytes(ctx, op.Path)
			if err != nil {
				log.Error().Msgf("reading record from event blocks (CAR): %s", err)
				continue
			}
			lexLinked := lexutil.LexLink(rc)
			log.Info().Msgf("lexLinked: %s", lexLinked)
			if op.Cid == nil || lexutil.LexLink(rc) != *op.Cid {
				log.Error().Msgf("mismatch between commit op CID and record block: recordCID: %s, op cid: %s", rc, op.Cid)
				continue
			}

			recordATURI := fmt.Sprintf("at://%s/%s", evt.Repo, op.Path)
			ingestionChain := NewRecordChainIngestor(
				*recordCBOR,
				op.Cid.String(),
				recordATURI,
				f.atprotoEventPublisher,
			)

			f.ingester.EnqueueChain(ingestionChain)

		default:
			// ignore other events
		}
	}

	return nil
}
