package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller/encrypter"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

// TODO: An integration test with PDS running + real encryption
// This mocks out the PDS and uses a no-op encrypter
func TestControllerPrivateDataPutGet(t *testing.T) {
	ctx := context.Background()
	encrypter, err := encrypter.NewFromKey([]byte(encrypter.TestOnlyNewRandomKey()))
	require.NoError(t, err)

	type req struct {
		url   string
		req   []byte
		resp  []byte
		isErr bool
	}

	// The val the caller is trying to put
	val := map[string]any{
		"someKey": "someVal",
	}
	//cidLink := util.LexLink("my-cid")
	blobCid := "bafkreibznq4kuh7vx6wwmgfbgu6wsjbmnws6tyhasnobdhuflh3vgl6ye4"
	blob1 := atproto.RepoUploadBlob_Output{
		Blob: &util.LexBlob{
			Ref: util.LexLink(cid.MustParse(blobCid)),
		},
	}
	resp1, err := json.Marshal(blob1)
	require.NoError(t, err)

	encRecordCid := "bafkreibznq4kuh7vx6wwmgfbgu6wsjbmnws6tyhasnobdhuflh3vgl6ye5"
	testUri := "testUri"
	put2 := atproto.RepoPutRecord_Output{
		Cid: encRecordCid,
		Uri: testUri,
	}
	resp2, err := json.Marshal(put2)
	require.NoError(t, err)

	xrpcErr := xrpc.XRPCError{
		ErrStr:  "Could not locate record",
		Message: "Could not locate record",
	}
	recordNotFoundResp, err := json.Marshal(xrpcErr)
	require.NoError(t, err)

	resp4 := []byte(fmt.Sprintf(`{
		"cid": "%s", 
		"uri": "%s", 
		"value": {
			"$type": "com.habitat.encryptedRecord",
			"cid": "%s"
		}
	}`, encRecordCid, testUri, blobCid))

	marshalledVal, err := json.Marshal(val)
	require.NoError(t, err)
	encrypted, err := encrypter.Encrypt("my-rkey", marshalledVal)
	require.NoError(t, err)
	resp5 := encrypted

	reqOrder := []req{
		{
			url:   "/xrpc/com.atproto.repo.getRecord?cid=&collection=my.fake.collection&repo=my-did&rkey=my-rkey",
			resp:  recordNotFoundResp,
			isErr: true,
		},
		{
			url:  "/xrpc/com.atproto.repo.uploadBlob",
			resp: resp1,
		},
		{
			url:  "/xrpc/com.atproto.repo.putRecord",
			resp: resp2,
		},
		{
			url:   "/xrpc/com.atproto.repo.getRecord?cid=&collection=my.fake.collection&repo=my-did&rkey=my-rkey",
			resp:  recordNotFoundResp,
			isErr: true,
		},
		{
			url:  "/xrpc/com.atproto.repo.getRecord?cid=&collection=com.habitat.encryptedRecord&repo=my-did&rkey=enc%3Amy.fake.collection%3Amy-rkey",
			resp: resp4,
		},
		{
			url:  fmt.Sprintf("/xrpc/com.atproto.sync.getBlob?cid=%s&did=my-did", blobCid),
			resp: resp5,
		},
	}
	curr := 0

	mockPDS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expReq := reqOrder[curr]
		require.Equal(t, expReq.url, r.URL.String())

		if expReq.isErr {
			w.WriteHeader(http.StatusForbidden)
		}
		w.Write(expReq.resp)
		curr += 1
	}))
	defer mockPDS.Close()

	mockDriver := newMockDriver(node.DriverTypeDocker)
	pm := process.NewProcessManager([]process.Driver{mockDriver})

	ctrl, err := NewController2(
		context.Background(),
		pm,
		nil,
		&mockHDB{
			schema:    state.Schema(),
			jsonState: jsonStateFromNodeState(state),
		},
		nil, /* reverse proxy */
		&xrpc.Client{
			Host: mockPDS.URL,
		},
		encrypter,
	)

	// putRecord with encryption
	coll := "my.fake.collection"
	out, err := ctrl.putRecord(ctx, &agnostic.RepoPutRecord_Input{
		Collection: coll,
		Record:     val,
		Repo:       "my-did",
		Rkey:       "my-rkey",
	}, true)
	require.NoError(t, err)
	require.Equal(t, encRecordCid, out.Cid)
	require.Equal(t, testUri, out.Uri)

	got, err := ctrl.getRecord(ctx, "", coll, "my-did", "my-rkey")
	require.NoError(t, err)
	require.Equal(t, *got.Cid, encRecordCid)
	require.Equal(t, got.Uri, testUri)
	bytes, err := got.Value.MarshalJSON()
	require.Equal(t, bytes, marshalledVal)
}
