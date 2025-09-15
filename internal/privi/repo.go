package privi

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// Persist private data within repos that mirror public repos.
// A repo implements two methods: putRecord and getRecord.
// In the future, it is possible to implement sync endpoints and other methods.

type repo interface {
	putRecord(did string, rkey string, rec record, validate *bool) error
	getRecord(did string, rkey string) (record, error)
}

// Lexicon NSID -> records for that lexicon.
// A record is stored as raw bytes and keyed by its record key (rkey).
//
// TODO: the internal store should be an MST for portability / compatiblity with conventional atproto  methods.
type inMemoryRepo map[syntax.DID]map[recordKey]record

// inMemoryRepo implements repo
var _ repo = &inMemoryRepo{}

func newInMemoryRepo() inMemoryRepo {
	return make(inMemoryRepo)
}

// putRecord puts a record for the given rkey into the repo no matter what; if a record always exists, it is overwritten.
func (r inMemoryRepo) putRecord(did string, rkey string, rec record, validate *bool) error {
	if validate != nil && *validate {
		err := data.Validate(rec)
		if err != nil {
			return err
		}
	}

	coll, ok := r[syntax.DID(did)]
	if !ok {
		coll = make(map[recordKey]record)
		r[syntax.DID(did)] = coll
	}

	// Always put (even if something exists).
	coll[recordKey(rkey)] = rec
	return nil
}

func (r inMemoryRepo) getRecord(did string, rkey string) (record, error) {
	coll, ok := r[syntax.DID(did)]
	if !ok {
		return nil, ErrRecordNotFound
	}

	record, ok := coll[recordKey(rkey)]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return record, nil
}

// A sqlite-backed repo per user contains the following two columns:
// [did, record key, record value]
// For now, store all records in the same database. Eventually, this should be broken up into
// per-user databases or per-user MST repos.

type sqliteRepo struct {
	db *sql.DB
}

var _ repo = &sqliteRepo{}

// Shape of a row in the sqlite table
type row struct {
	did  string
	rkey string
	rec  string
}

// TODO: create table etc.
func NewSQLiteRepo(db *sql.DB) repo {
	return &sqliteRepo{
		db: db,
	}
}

// putRecord puts a record for the given rkey into the repo no matter what; if a record always exists, it is overwritten.
func (r *sqliteRepo) putRecord(did string, rkey string, rec record, validate *bool) error {
	if validate != nil && *validate {
		err := data.Validate(rec)
		if err != nil {
			return err
		}
	}

	bytes, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	// Always put (even if something exists).
	_, err = r.db.Exec("insert into records(did, rkey, record) values(?, ?, ?);", did, rkey, string(bytes))
	return err
}

var (
	ErrRecordNotFound       = fmt.Errorf("record not found")
	ErrMultipleRecordsFound = fmt.Errorf("multiple records found for desired query")
)

func (r *sqliteRepo) getRecord(did string, rkey string) (record, error) {
	rows, err := r.db.Query("select * from records where rkey = ? and did = ?", rkey, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var row row
	if rows.Next() {
		err = rows.Scan(&row.did, &row.rkey, &row.rec)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrRecordNotFound
	}

	if rows.Next() {
		return nil, ErrMultipleRecordsFound
	}

	var record map[string]any
	err = json.Unmarshal([]byte(row.rec), &record)
	if err != nil {
		return nil, err
	}

	// TODO: return ErrorRecordNotFound somewhere
	return record, nil
}
