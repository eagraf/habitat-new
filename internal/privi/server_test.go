package privi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/eagraf/habitat-new/api/habitat"
	"github.com/eagraf/habitat-new/internal/permissions"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestServerUploadBlob(t *testing.T) {
	// use in-memory sqlite for test
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo, err := NewSQLiteRepo(db)
	require.NoError(t, err)
	// test environment may report 0 for compile-time MAX_LENGTH; allow some reasonable size for test
	repo.blobMaxSize = 1024 * 1024

	perms := permissions.NewDummyStore()
	server := NewServer(perms, repo)

	// create request with blob body
	body := []byte("test-blob")
	req := httptest.NewRequest(http.MethodPost, "/xrpc/com.habitat.uploadBlob", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	// Call UploadBlob handler directly with a synthetic caller DID
	h := server.UploadBlob(syntax.DID("did:example:alice"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	// decode response
	var out habitat.NetworkHabitatRepoUploadBlobOutput
	err = json.Unmarshal(rr.Body.Bytes(), &out)
	require.NoError(t, err)
	require.NotNil(t, out.Blob)
	// out.Blob is an interface{} (decoded as map[string]any)
	blobMap, ok := out.Blob.(map[string]any)
	require.True(t, ok, "expected blob to decode to map")

	// mimetype
	mt, ok := blobMap["mimetype"].(string)
	require.True(t, ok)
	require.Equal(t, "text/plain", mt)

	// size (json decodes numbers as float64)
	szf, ok := blobMap["size"].(float64)
	require.True(t, ok)
	require.Equal(t, float64(len(body)), szf)

	// cid is encoded as an object with $link: {"$link":"<cid>"}
	cidObj, ok := blobMap["cid"].(map[string]any)
	require.True(t, ok)
	cidVal, ok := cidObj["$link"].(string)
	require.True(t, ok)
	cidStr := cidVal

	// verify repo stored it
	mimetype, gotBlob, err := repo.getBlob("did:example:alice", cidStr)
	require.NoError(t, err)
	require.Equal(t, "text/plain", mimetype)
	require.Equal(t, body, gotBlob)
}
