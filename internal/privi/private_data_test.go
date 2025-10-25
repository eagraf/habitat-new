package privi

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/eagraf/habitat-new/internal/permissions"
	"github.com/stretchr/testify/require"
)

// A unit test testing putRecord and getRecord with one basic permission.
// TODO: an integration test with two PDS's + privi servers running.
func TestControllerPrivateDataPutGet(t *testing.T) {
	// The val the caller is trying to put
	val := map[string]any{
		"someKey": "someVal",
	}
	marshalledVal, err := json.Marshal(val)
	require.NoError(t, err)

	dummy := permissions.NewDummyStore()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	repo, err := NewSQLiteRepo(db)
	require.NoError(t, err)
	p := newStore(dummy, repo)

	// putRecord
	coll := "my.fake.collection"
	rkey := "my-rkey"
	validate := true
	err = p.putRecord("my-did", coll, val, rkey, &validate)
	require.NoError(t, err)

	got, err := p.getRecord(coll, rkey, "my-did", "another-did")
	require.Nil(t, got)
	require.ErrorIs(t, ErrUnauthorized, err)

	require.NoError(t, dummy.AddLexiconReadPermission("another-did", "my-did", coll))

	got, err = p.getRecord(coll, "my-rkey", "my-did", "another-did")
	require.NoError(t, err)

	marshalled, err := json.Marshal(got)
	require.NoError(t, err)
	require.Equal(t, []byte(marshalled), marshalledVal)

	err = p.putRecord("my-did", coll, val, rkey, &validate)
	require.NoError(t, err)
}

func TestGetBlob(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo, err := NewSQLiteRepo(db)
	require.NoError(t, err)
	// ensure reasonable blob limit in test env
	repo.maxBlobSize = 1024 * 1024

	did := "did:example:alice"
	data := []byte("hello world")
	mtype := "text/plain"

	meta, err := repo.uploadBlob(did, data, mtype)
	require.NoError(t, err)

	// meta.Ref is a atdata.CIDLink which prints nested structure; use its String() method
	cidStr := meta.Ref.String()

	gotMtype, gotData, err := repo.getBlob(did, cidStr)
	require.NoError(t, err)
	require.Equal(t, mtype, gotMtype)
	require.Equal(t, data, gotData)
}
