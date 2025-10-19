package privi

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/eagraf/habitat-new/api/habitat"
	"github.com/stretchr/testify/require"
)

func TestSQLiteRepoPutAndGetRecord(t *testing.T) {
	testDBPath := filepath.Join(os.TempDir(), "test_privi.db")
	defer os.Remove(testDBPath)

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

func TestSQLiteRepoListRecords(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	repo, err := NewSQLiteRepo(db)
	require.NoError(t, err)

	err = repo.putRecord(
		"my-did",
		"network.habitat.collection-1.key-1",
		map[string]any{"data": "value"},
		nil,
	)
	require.NoError(t, err)

	err = repo.putRecord(
		"my-did",
		"network.habitat.collection-1.key-2",
		map[string]any{"data": "value"},
		nil,
	)
	require.NoError(t, err)

	err = repo.putRecord(
		"my-did",
		"network.habitat.collection-2.key-2",
		map[string]any{"data": "value"},
		nil,
	)
	require.NoError(t, err)

	records, err := repo.listRecords(
		habitat.NetworkHabitatRepoListRecordsParams{
			Repo:       "my-did",
			Collection: "my-collection",
		},
		[]string{},
		[]string{},
	)
	require.NoError(t, err)
	require.Len(t, records, 0)

	records, err = repo.listRecords(
		habitat.NetworkHabitatRepoListRecordsParams{
			Repo:       "my-did",
			Collection: "my-collection",
		},
		[]string{"network.habitat.collection-1.key-1", "network.habitat.collection-1.key-2"},
		[]string{},
	)
	require.NoError(t, err)
	require.Len(t, records, 2)

	records, err = repo.listRecords(
		habitat.NetworkHabitatRepoListRecordsParams{
			Repo:       "my-did",
			Collection: "my-collection",
		},
		[]string{"network.habitat.collection-1.*"},
		[]string{},
	)
	require.NoError(t, err)
	require.Len(t, records, 2)

	records, err = repo.listRecords(
		habitat.NetworkHabitatRepoListRecordsParams{
			Repo:       "my-did",
			Collection: "my-collection",
		},
		[]string{"network.habitat.collection-1.*"},
		[]string{"network.habitat.collection-1.key-1"},
	)
	require.NoError(t, err)
	require.Len(t, records, 1)

	records, err = repo.listRecords(
		habitat.NetworkHabitatRepoListRecordsParams{
			Repo:       "my-did",
			Collection: "my-collection",
		},
		[]string{"network.habitat.*"},
		[]string{"network.habitat.collection-1.key-1"},
	)
	require.NoError(t, err)
	require.Len(t, records, 2)
}
