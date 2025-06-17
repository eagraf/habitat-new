package privi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/eagraf/habitat-new/core/permissions"
)

type record map[string]any

// A record key is a string
type recordKey string

// Lexicon NSID -> records for that lexicon.
// A record is stored as raw bytes and keyed by its record key (rkey).
//
// TODO: the internal store should be an MST for portability / compatiblity with conventional atproto  methods.
type inMemoryRepo map[syntax.NSID]map[recordKey]record

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

	if _, exists := coll[recordKey(rkey)]; exists {
		// TODO; are all puts legal?
	}

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

// Privi is an ATProto PDS Wrapper which allows for storing & getting private data.
// It does this by encrypting data, then storing it in blob. A special lexicon for this purpose,
// identified by com.habitat.encryptedRecord, points to the blob storing the actual data.
//
// This encryption layer is transparent to the caller -- Privi.PutRecord() and Privi.GetRecord() have
// the same API has com.atproto.putRecord and com.atproto.getRecord.
//
// TODO: formally define the com.habitat.encryptedRecord and change it to a domain we actually own :)
type store struct {
	did syntax.DID
	// TODO: consider encrypting at rest. probably not, and construct a wholly separate MST for private data.
	// e           Encrypter
	permissions permissions.Store

	// TODO: this should be a portable MST the same as stored in the PDS. For ease/demo purposes, just use an
	// in-memory store.
	repo inMemoryRepo
}

const encryptedRecordNSID = "com.habitat.encryptedRecord"

func encryptedRecordRKey(collection string, rkey string) string {
	return fmt.Sprintf("enc:%s:%s", collection, rkey)
}

type encryptedRecord struct {
	Data util.BlobSchema `json:"data" cborgen:"data"`
}

var (
	ErrPublicRecordExists               = fmt.Errorf("a public record exists with the same key")
	ErrNoPutsOnEncryptedRecord          = fmt.Errorf("directly put-ting to this lexicon is not valid")
	ErrNoEncryptedGetsOnEncryptedRecord = fmt.Errorf("calling getEncryptedRecord on a %s is not supported", encryptedRecordNSID)
	ErrEncryptedRecordNilValue          = fmt.Errorf("a %s record was found but it has a nil value", encryptedRecordNSID)
	ErrNotLocalRepo                     = fmt.Errorf("the desired did does not live on this repo")
	ErrUnauthorized                     = fmt.Errorf("unauthorized request")
	ErrRecordNotFound                   = fmt.Errorf("record not found")
)

// Returns true if err indicates the RecordNotFound error
func errorIsNoRecordFound(err error) bool {
	// TODO: Not sure if the atproto lib has an error to directly use with errors.Is()
	return strings.Contains(err.Error(), "RecordNotFound") || strings.Contains(err.Error(), "Could not locate record")
}

// TODO: take in a carfile/sqlite where user's did is persisted
func newStore(did syntax.DID, perms permissions.Store) *store {
	return &store{
		did:         did,
		permissions: perms,
		repo:        make(inMemoryRepo),
	}
}

// type encryptedRecord map[string]any
// the shape of the lexicon is { "cid": <cid pointing to the encrypted blob> }

// putRecord with encryption wrapper around this
// ONLY YOU CAN CALL PUT RECORD, NO ONE ELSE
// Our security relies on this -- if this wasn't true then theoretically an attacker could call putRecord trying different rkey.
// If they were unable to create with an rkey, that means that it exists privately.
func (p *store) putRecord(collection string, record map[string]any, rkey string, validate *bool) error {
	// It is assumed right now that if this endpoint is called, the caller wants to put a private record into privi.
	return p.repo.putRecord(collection, record, rkey, validate)
}

type GetRecordResponse struct {
	Cid   *string `json:"cid"`
	Uri   string  `json:"uri"`
	Value any     `json:"value"`
}

// TODO: getRecord via cid -- depends on MST implementation
func (p *store) getRecord(collection string, rkey string, callerDID syntax.DID) (json.RawMessage, error) {
	// Run permissions before returning to the user
	authz, err := p.permissions.HasPermission(callerDID.String(), collection, rkey, false)
	if err != nil {
		return nil, err
	}

	if !authz {
		fmt.Println("caller", callerDID, "is not authorized to ", p.did, "collection", collection)
		return nil, ErrUnauthorized
	}

	record, ok := p.repo.getRecord(collection, rkey)
	if !ok {
		// TODO: is this the right thing to return here?
		return nil, ErrRecordNotFound
	}

	raw, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
