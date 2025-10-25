package privi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bradenaw/juniper/xslices"
	"github.com/eagraf/habitat-new/util"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/rs/zerolog/log"
)

// Persist private data within repos that mirror public repos.
// A repo currently implements four basic methods: putRecord, getRecord, uploadBlob, getBlob
// In the future, it is possible to implement sync endpoints and other methods.

// A sqlite-backed repo per user contains the following two columns:
// [did, record key, record value]
// For now, store all records in the same database. Eventually, this should be broken up into
// per-user databases or per-user MST repos.

// We really shouldn't have unexported types that get passed around outside the package, like to `main.go`
// Leaving this as-is for now.
type sqliteRepo struct {
	db          *sql.DB
	maxBlobSize int
}

// Helper function to query sqlite compile-time options to get the max blob size
// Not sure if this can change across versions, if so we need to keep that stable
func getMaxBlobSize(db *sql.DB) (int, error) {
	rows, err := db.Query("PRAGMA compile_options;")
	defer util.Close(rows, func(err error) {
		log.Err(err).Msgf("error closing db rows")
	})

	if err != nil {
		return 0, nil
	}

	for rows.Next() {
		var opt string
		_ = rows.Scan(&opt)
		if strings.HasPrefix(opt, "MAX_LENGTH=") {
			return strconv.Atoi(strings.TrimPrefix(opt, "MAX_LENGTH="))
		}
	}
	return 0, fmt.Errorf("no MAX_LENGTH parameter found")
}

func NewSQLiteRepo(db *sql.DB) (*sqliteRepo, error) {
	// Create records table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		did TEXT NOT NULL,
		rkey TEXT NOT NULL,
		nsid TEXT NOT NULL,
		record BLOB,
	);`)
	if err != nil {
		return nil, err
	}

	// Create index for efficient fetching on did + rkey
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS index_records_by_rkey ON records (did, rkey)`)
	if err != nil {
		return nil, err
	}

	// Create blobs table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS blobs (
		did TEXT NOT NULL,
		cid TEXT NOT NULL,
		mimetype TEXT,
		blob BLOB,
		PRIMARY KEY(cid)
	);`)
	if err != nil {
		return nil, err
	}

	// Create table mapping blob -> records that reference them
	// Columns are: cid | did | rkey, where cid uniquely identifies the blob and did + rkey uniquely identify a record that references it
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS blob_refs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		record_id INTEGER NOT NULL,
		cid TEXT NOT NULL,
		did TEXT NOT NULL
	);`)
	if err != nil {
		return nil, err
	}

	// Index the blob_refs table by cid + did for looking up blob -> record mapping
	_, err = db.Exec(`CREATE INDEX index_blob_refs_by_cid ON blob_refs (cid, did)`)
	if err != nil {
		return nil, err
	}

	// Index the blob_refs table by record_id for deletion upon record deletion
	_, err = db.Exec(`CREATE INDEX index_blob_refs_by_record ON blob_refs (record_id)`)
	if err != nil {
		return nil, err
	}

	maxBlobSize, err := getMaxBlobSize(db)
	if err != nil {
		return nil, err
	}

	return &sqliteRepo{
		db:          db,
		maxBlobSize: maxBlobSize,
	}, nil
}

// TODO: does this need to recurse more than one layer?
func getBlobRefsFromSchema(sch *lexicon.SchemaFile, rec record) ([]syntax.CID, error) {
	cids := make([]syntax.CID, 0)
	for key, def := range sch.Defs {
		if _, ok := def.Inner.(lexicon.SchemaCIDLink); ok {
			cid, ok := rec[key].(string)
			if !ok {
				return nil, fmt.Errorf("error type-casting $link in record to cid link string; path:", rec[key])
			}
			cids = append(cids, syntax.CID(cid))
		}
	}
	return cids, nil
}

// putRecord puts a record for the given rkey into the repo no matter what; if a record always exists, it is overwritten.
func (r *sqliteRepo) putRecord(did string, rkey string, rec record, nsid string, validate *bool) error {
	// TODO: this doesn't actually validate that the record is well-formed against nsid
	if validate != nil && *validate {
		err := atdata.Validate(rec)
		if err != nil {
			return err
		}
	}

	// TODO: Recursively traverse rec to determine if it references a blob, and if so, add it to blob_refs
	sch, err := lexicon.ResolveLexiconSchemaFile(context.Background(), identity.DefaultDirectory(), syntax.NSID(nsid))
	if err != nil {
		return err
	}

	blobRefs, err := getBlobRefsFromSchema(sch, rec)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	// Always put (even if something exists).
	var id int
	err = tx.QueryRow(
		"insert or replace into records(did, rkey, record, nsid) values(?, ?, ?, jsonb(?)) returning id;",
		did,
		rkey,
		nsid,
		bytes,
	).Scan(&id)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, cid := range blobRefs {
		_, err = tx.Exec("insert into blob_refs (record_id, cid, did) values (?, ?, ?)", id, cid, did)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

var (
	ErrRecordNotFound       = fmt.Errorf("record not found")
	ErrMultipleRecordsFound = fmt.Errorf("multiple records found for desired query")
)

func (r *sqliteRepo) getRecord(did string, rkey string) (record, error) {
	queried := r.db.QueryRow(
		"select did, rkey, json(record) from records where rkey = ? and did = ?",
		rkey,
		did,
	)

	var row struct {
		did  string
		rkey string
		rec  string
	}
	err := queried.Scan(&row.did, &row.rkey, &row.rec)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	} else if err != nil {
		return nil, err
	}

	var record record
	err = json.Unmarshal([]byte(row.rec), &record)
	if err != nil {
		return nil, err
	}

	return record, nil
}

type blob struct {
	Ref      atdata.CIDLink `json:"cid"`
	MimeType string         `json:"mimetype"`
	Size     int64          `json:"size"`
}

func (r *sqliteRepo) uploadBlob(did string, data []byte, mimeType string) (*blob, error) {
	// Validate blob size
	if len(data) > r.maxBlobSize {
		return nil, fmt.Errorf("blob size is too big, must be < max blob size (based on SQLITE MAX_LENGTH compile option): %d bytes", r.maxBlobSize)
	}

	// "blessed" CID type: https://atproto.com/specs/blob#blob-metadata
	cid, err := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256).Sum(data)
	if err != nil {
		return nil, err
	}

	sql := `insert into blobs(did, cid, mimetype, blob) values(?, ?, ?, ?)`
	_, err = r.db.Exec(sql, did, cid.String(), mimeType, data)
	if err != nil {
		return nil, err
	}

	return &blob{
		Ref:      atdata.CIDLink(cid),
		MimeType: mimeType,
		Size:     int64(len(data)),
	}, nil
}

// getBlob gets a blob. this is never exposed to the server, because blobs can only be resolved via records that link them (see LexLink)
// besides exceptional cases like data migration which we do not support right now.
func (r *sqliteRepo) getBlob(did string, cid string) (string /* mimetype */, []byte /* raw blob */, error) {
	qry := "select mimetype, blob from blobs where did = ? and cid = ?"
	res := r.db.QueryRow(
		qry,
		did,
		cid,
	)

	var mimetype string
	var blob []byte
	err := res.Scan(&mimetype, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrRecordNotFound
	} else if err != nil {
		return "", nil, err
	}

	return mimetype, blob, nil
}

// uniqeley identifies a record
type recordRef struct {
	ownerDID string
	rkey     string
	nsid     string
}

// Gets all the records that reference a blob given by cid
func (r *sqliteRepo) getBlobRefs(cid string, did string) ([]recordRef, error) {
	qry := "select record_id from blob_refs where cid = ? and did = ?"
	rows, err := r.db.Query(
		qry,
		cid,
		did,
	)
	if err != nil {
		return nil, err
	}

	ids := make([]int, 0)
	defer util.Close(rows)
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	// Now get the recordRefs for each returned record_id
	qs := make([]string, len(ids))
	xslices.Fill(qs, "?")
	qry2 := fmt.Sprintf("select (did, nsid, rkey) from records where record_id in (%s)", strings.Join(qs, ","))
	rows2, err := r.db.Query(qry2, ids)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows2)

	res := make([]recordRef, len(ids))
	for rows.Next() {
		var ref recordRef
		err = rows.Scan(&ref.ownerDID, ref.nsid, ref.rkey)
		if err != nil {
			return nil, err
		}
		res = append(res, ref)
	}

	return res, nil
}
