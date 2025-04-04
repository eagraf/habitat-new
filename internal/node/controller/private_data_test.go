package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	cbg "github.com/whyrusleeping/cbor-gen"
)

type testCborMarshaler struct {
	val []byte
}

func (t testCborMarshaler) MarshalCBOR(io.Writer) error {
	return nil
}

var _ cbg.CBORMarshaler = testCborMarshaler{}

// TODO: An integration test with PDS running + real encryption
// This mocks out the PDS and uses a no-op encrypter
func TestControllerPrivateDataPutGet(t *testing.T) {
	ctx := context.Background()

	type req struct {
		url     string
		req     []byte
		resp    []byte
		respErr *error
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
	/*

		blob := &util.BlobSchema{
			LexiconTypeID: "my.fake.collection",
			Ref:           util.LexLink(cid.MustParse(blobCid)),
		}
		lexval := &util.LexiconTypeDecoder{
			Val: blob,
		}
			get3 := atproto.RepoGetRecord_Output{
				Cid:   &blobCid,
				Uri:   testUri,
				Value: lexval,
			}

			//resp3, err := json.Marshal(get3)
			//require.NoError(t, err)
	*/
	reqOrder := []req{
		{
			url:  "/xrpc/com.atproto.repo.uploadBlob",
			resp: resp1,
		},
		{
			url:  "/xrpc/com.atproto.repo.putRecord",
			resp: resp2,
		},
		{
			url:     "/xrpc/com.atproto.repo.getRecord?cid=&collection=my.fake.collection&repo=my-did&rkey=my-rkey",
			resp:    []byte("Could not locate record"),
			respErr: &(fmt.Errorf("Could not locate record")),
		},
	}
	curr := 0

	mockPDS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expReq := reqOrder[curr]
		require.Equal(t, expReq.url, r.URL.String())
		w.Write(expReq.resp)
		curr += 1
	}))
	defer mockPDS.Close()

	mockDriver := newMockDriver(node.DriverTypeDocker)
	pm := process.NewProcessManager([]process.Driver{mockDriver})

	encrypter, err := encrypter.NewFromKey([]byte(encrypter.TestOnlyNewRandomKey()))
	require.NoError(t, err)
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
	require.NoError(t, err)

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
	require.Equal(t, *got.Cid, blobCid)
	require.Equal(t, got.Uri, testUri)
	bytes, err := got.Value.MarshalJSON()
	require.NoError(t, err)
	fmt.Println(string(bytes))

	/*
		// TODO: putRecord no encryption
		out, err = ctrl.putRecord(ctx, &agnostic.RepoPutRecord_Input{
			Collection: coll,
			Record:     val,
			Repo:       "my-did",
			Rkey:       "my-rkey",
		}, false)
		require.NoError(t, err)

		resp, err = ctrl.getRecord(ctx, out.Cid, coll, "my-did", "my-rkey")
		require.NoError(t, err)
		require.Equal(t, resp.Cid, out.Cid)
		require.Equal(t, val, resp.Value)
	*/
}
