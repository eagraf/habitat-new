package privi

import (
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

type record map[string]any

// A record key is a string
type recordKey string

type Repo interface {
	putRecord(collection string, rec record, rkey string, validate *bool) error
	getRecord(collection string, rkey string) (record, bool)
}

// Lexicon NSID -> records for that lexicon.
// A record is stored as raw bytes and keyed by its record key (rkey).
//
// TODO: the internal store should be an MST for portability / compatiblity with conventional atproto  methods.
type inMemoryRepo map[syntax.NSID]map[recordKey]record

// putRecord puts a record for the given rkey into the repo no matter what; if a record always exists, it is overwritten.
func (r inMemoryRepo) putRecord(collection string, rec record, rkey string, validate *bool) error {
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

func (r inMemoryRepo) getRecord(collection string, rkey string) (record, bool) {
	coll, ok := r[syntax.NSID(collection)]
	if !ok {
		return nil, false
	}

	record, ok := coll[recordKey(rkey)]
	return record, ok
}
