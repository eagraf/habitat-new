package privi

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteRepoPutAndGetRecord(t *testing.T) {
	testDBPath := filepath.Join(os.TempDir(), "test_privi.db")
	defer func() { require.NoError(t, os.Remove(testDBPath)) }()

	priviDB, err := sql.Open("sqlite3", testDBPath)
	require.NoError(t, err)

	repo, err := NewSQLiteRepo(priviDB)
	require.NoError(t, err)

	key := "test-key"
	val := map[string]any{"data": "value", "data-1": float64(123), "data-2": true}

	err = repo.putRecord("my-did", key, val, nil)
	require.NoError(t, err)

	got, err := repo.getRecord("my-did", key)
	require.NoError(t, err)

	for k, v := range val {
		_, ok := got[k]
		require.True(t, ok)
		require.Equal(t, got[k], v)
	}
}

func TestUploadAndGetBlob(t *testing.T) {
	testDBPath := filepath.Join(os.TempDir(), "test_privi_blobs.db")
	defer func() { require.NoError(t, os.Remove(testDBPath)) }()

	db, err := sql.Open("sqlite3", testDBPath)
	require.NoError(t, err)
	defer db.Close()

	repo, err := NewSQLiteRepo(db)
	require.NoError(t, err)

	sr, ok := repo.(*sqliteRepo)
	require.True(t, ok, "expected sqliteRepo implementation from NewSQLiteRepo")

	did := "did:example:alice"
	// use an empty blob to avoid hitting sqlite3.SQLITE_LIMIT_LENGTH in test environment
	blob := []byte("this is my test blob")
	mtype := "text/plain"

	bmeta, err := sr.uploadBlob(did, blob, mtype)
	require.NoError(t, err)
	require.NotNil(t, bmeta)
	require.Equal(t, mtype, bmeta.MimeType)
	require.Equal(t, int64(len(blob)), bmeta.Size)

	m, gotBlob, err := sr.getBlob(did, bmeta.Ref.String())
	require.NoError(t, err)
	require.Equal(t, mtype, m)
	require.Equal(t, blob, gotBlob)
}
