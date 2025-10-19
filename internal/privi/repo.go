package privi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/eagraf/habitat-new/api/habitat"
	_ "github.com/mattn/go-sqlite3"

	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"

	sq "github.com/Masterminds/squirrel"
)

// Persist private data within repos that mirror public repos.
// A repo implements two methods: putRecord and getRecord.
// In the future, it is possible to implement sync endpoints and other methods.

type repo interface {
	putRecord(did string, rkey string, rec record, validate *bool) error
	getRecord(did string, rkey string) (record, error)
	listRecords(
		params habitat.NetworkHabitatRepoListRecordsParams,
		allow []string,
		deny []string,
	) ([]record, error)
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

func (r inMemoryRepo) listRecords(
	params habitat.NetworkHabitatRepoListRecordsParams,
	allow []string,
	deny []string,
) ([]record, error) {
	coll, ok := r[syntax.DID(params.Repo)]
	if !ok {
		return nil, ErrRecordNotFound
	}

	result := []record{}

	for rkey, record := range coll {
		for _, d := range deny {
			if strings.HasPrefix(string(rkey), strings.TrimSuffix(d, "*")) {
				continue
			}
		}
		for _, a := range allow {
			if strings.HasPrefix(string(rkey), strings.TrimSuffix(a, "*")) {
				result = append(result, record)
			}
		}
	}
	return nil, ErrRecordNotFound
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
func NewSQLiteRepo(db *sql.DB) (repo, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS records (
		did TEXT NOT NULL,
		rkey TEXT NOT NULL,
		record BLOB,
		PRIMARY KEY(did, rkey)
	);`)
	if err != nil {
		return nil, err
	}
	return &sqliteRepo{
		db: db,
	}, nil
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
	_, err = r.db.Exec(
		"insert into records(did, rkey, record) values(?, ?, jsonb(?));",
		did,
		rkey,
		bytes,
	)
	return err
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

	var row row
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

// listRecords implements repo.
func (r *sqliteRepo) listRecords(
	params habitat.NetworkHabitatRepoListRecordsParams,
	allow []string,
	deny []string,
) ([]record, error) {
	query := sq.Select("record").From("records").Where(sq.Eq{"did": params.Repo})
	for _, d := range deny {
		query = query.Where(sq.NotLike{"rkey": strings.TrimSuffix(d, "*")})
	}
	for _, a := range allow {
		query = query.Where(sq.Like{"rkey": strings.TrimSuffix(a, "*")})
	}
	if params.Cursor != "" {
		query = query.Where(sq.Gt{"rkey": params.Cursor})
	}
	if params.Limit != 0 {
		query = query.Limit(uint64(params.Limit))
	}
	rows, err := query.RunWith(r.db).Query()
	if err != nil {
		return nil, err
	}
	records := []record{}
	for rows.Next() {
		var rec string
		if err := rows.Scan(&rec); err != nil {
			return nil, err
		}
		var record record
		if err := json.Unmarshal([]byte(rec), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
