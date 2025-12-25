package privi

import (
	"context"
	"encoding/json"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
)

type record map[string]any

// A record key is a string
type recordKey string

type Repo interface {
	putRecord(ctx context.Context, did syntax.DID, collection string, record record, rkey string, validate *bool) error
	getRecord(ctx context.Context, did syntax.DID, collection string, rkey string) (*record, error)
}

// Lexicon NSID -> records for that lexicon.
// A record is stored as raw bytes and keyed by its record key (rkey).
//
// TODO: the internal store should be an MST for portability / compatiblity with conventional atproto  methods.
type inMemoryRepo map[syntax.NSID]map[recordKey]record

var _ Repo = (*inMemoryRepo)(nil)

// putRecord puts a record for the given rkey into the repo no matter what; if a record always exists, it is overwritten.
func (r inMemoryRepo) putRecord(_ context.Context, _ syntax.DID, collection string, rec record, rkey string, validate *bool) error {
	if validate != nil && *validate {
		err := data.Validate(rec)
		if err != nil {
			return err
		}
	}

	coll, ok := r[syntax.NSID(collection)]
	if !ok {
		coll = make(map[recordKey]record)
		r[syntax.NSID(collection)] = coll
	}

	// Always put (even if something exists).
	coll[recordKey(rkey)] = rec
	return nil
}

func (r inMemoryRepo) getRecord(_ context.Context, _ syntax.DID, collection string, rkey string) (*record, error) {
	coll, ok := r[syntax.NSID(collection)]
	if !ok {
		return nil, nil
	}

	record, ok := coll[recordKey(rkey)]
	return &record, nil
}

type pdsRepoClient struct {
	inner *xrpc.Client
}

func NewPDSRepo(cli *xrpc.Client) Repo {
	return &pdsRepoClient{
		inner: cli,
	}
}

func (c *pdsRepoClient) putRecord(ctx context.Context, did syntax.DID, collection string, rec record, rkey string, validate *bool) error {
	_, err := agnostic.RepoPutRecord(ctx, c.inner, &agnostic.RepoPutRecord_Input{
		Collection: collection,
		Record:     rec,
		Repo:       did.Identifier(),
		Rkey:       rkey,
		Validate:   validate,
	})
	return err
}

func (c *pdsRepoClient) getRecord(ctx context.Context, did syntax.DID, collection string, rkey string) (*record, error) {
	out, err := agnostic.RepoGetRecord(ctx, c.inner, "" /* todo */, collection, did.Identifier(), rkey)
	if err != nil {
		return nil, err
	}

	var res *record
	err = json.Unmarshal(*out.Value, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
