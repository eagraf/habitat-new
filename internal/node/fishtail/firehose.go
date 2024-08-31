package fishtail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/rs/zerolog/log"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repomgr"

	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/repo"
	"github.com/gorilla/websocket"
)

type TailService interface {
}

type Fishtail struct {
	relayHost string
	pdsClient *controller.PDSClient
}

func NewFishtailService(relayHost string, nodeConfig *config.NodeConfig) *Fishtail {
	return &Fishtail{
		relayHost: relayHost,
		pdsClient: &controller.PDSClient{NodeConfig: nodeConfig},
	}
}

func (f *Fishtail) FirehoseConsumer(ctx context.Context, relayHost string) func() error {
	return func() error {
		return f.runFirehoseConsumer(
			ctx,
			relayHost,
		)
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

	// Hello

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

// TODO: move this to a "ParsePath" helper in syntax package?
func splitRepoPath(path string) (syntax.NSID, syntax.RecordKey, error) {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid record path: %s", path)
	}
	collection, err := syntax.ParseNSID(parts[0])
	if err != nil {
		return "", "", err
	}
	rkey, err := syntax.ParseRecordKey(parts[1])
	if err != nil {
		return "", "", err
	}
	return collection, rkey, nil
}

// NOTE: for now, this function basically never errors, just logs and returns nil. Should think through error processing better.
func (f *Fishtail) handleRepoCommit(ctx context.Context, evt *comatproto.SyncSubscribeRepos_Commit) error {

	log.Debug().Msgf("received commit event: %v", evt)

	if evt.TooBig {
		log.Warn().Msg("skipping tooBig events for now")
		return nil
	}

	did, err := syntax.ParseDID(evt.Repo)
	if err != nil {
		log.Error().Msgf("bad DID syntax in event %s", err)
		return nil
	}
	log.Info().Msgf("did: %s", did)

	rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
	if err != nil {
		log.Error().Msgf("failed to read repo from car %s", err)
		return nil
	}

	for _, op := range evt.Ops {
		collection, rkey, err := splitRepoPath(op.Path)
		if err != nil {
			log.Error().Msgf("invalid path in repo op: %s", err)
			return nil
		}
		log.Info().Msgf("collection: %s, rkey: %s", collection, rkey)

		ek := repomgr.EventKind(op.Action)
		switch ek {
		case repomgr.EvtKindCreateRecord, repomgr.EvtKindUpdateRecord:
			// read the record bytes from blocks, and verify CID
			rc, recordCBOR, err := rr.GetRecordBytes(ctx, op.Path)
			if err != nil {
				log.Error().Msgf("reading record from event blocks (CAR): %s", err)
				continue
			}
			if op.Cid == nil || lexutil.LexLink(rc) != *op.Cid {
				log.Error().Msgf("mismatch between commit op CID and record block: recordCID: %s, op cid: %s", rc, op.Cid)
				continue
			}

			log.Info().Msgf("Record CBOR bytes %s", fmt.Sprintf("%X", *recordCBOR))

			err = f.ingestLinkedRecords("fakeuri", *recordCBOR)
			if err != nil {
				log.Error().Msgf("error ingesting linked records: %s", err)
			}

		default:
			// ignore other events
		}
	}

	return nil
}

func (f *Fishtail) ingestLinkedRecords(uri syntax.ATURI, initialCBORRecord []byte) error {

	// The initial record we ingest is using CBOR because it comes from the event stream.

	// Use a map to track seen URIs to avoid duplication and ensure we don't get stuck in a loop
	seen := make(map[string]bool)
	seen[uri.String()] = true

	var linkedURIs []syntax.ATURI
	extractAtProtoURI := func(node interface{}) error {
		switch val := node.(type) {
		case string:
			if strings.HasPrefix(val, "at://") {
				log.Info().Msgf("found linked URI: %s", val)
				linkedURIs = append(linkedURIs, syntax.ATURI(val))
			}
			return nil
		default:
			return nil
		}
	}

	err := traverseCBOR(initialCBORRecord, extractAtProtoURI)
	if err != nil {
		return fmt.Errorf("failed to traverse CBOR: %w", err)
	}

	for _, uri := range linkedURIs {
		if err := f._ingestLinkedRecords(uri, seen); err != nil {
			return fmt.Errorf("failed to ingest linked record: %w", err)
		}
	}

	return nil
}

func (f *Fishtail) _ingestLinkedRecords(atURI syntax.ATURI, seen map[string]bool) error {
	// Fetch the record
	// Parse the ATURI
	log.Info().Msgf("ingesting linked record: %s", atURI)

	recordResp, err := f.pdsClient.GetRecord(
		atURI.Authority().String(),
		atURI.Collection().String(),
		atURI.RecordKey().String(),
	)
	if err != nil {
		return fmt.Errorf("failed to fetch record from PDS: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(recordResp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record to JSON: %w", err)
	}

	log.Info().Msgf("%s", jsonBytes)

	var linkedURIs []syntax.ATURI
	extractAtProtoURI := func(node interface{}) error {
		switch val := node.(type) {
		case string:
			if strings.HasPrefix(val, "at://") {
				log.Info().Msgf("found linked URI: %s", val)
				linkedURIs = append(linkedURIs, syntax.ATURI(val))
			}
			return nil
		default:
			return nil
		}
	}

	err = traverseJSON(recordResp.Value, extractAtProtoURI)
	if err != nil {
		return fmt.Errorf("failed to traverse JSON: %w", err)
	}

	// Recursively ingest linked records
	for _, uri := range linkedURIs {
		if !seen[string(uri)] {
			seen[string(uri)] = true
			if err := f._ingestLinkedRecords(uri, seen); err != nil {
				return fmt.Errorf("failed to ingest linked record: %w", err)
			}
		} else {
			log.Warn().Msgf("skipping duplicate linked record: %s", uri)
		}
	}

	return nil
}
