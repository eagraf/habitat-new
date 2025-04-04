package controller

import (
	"context"
	"testing"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller/encrypter"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/stretchr/testify/require"
)

// TODO: An integration test with PDS running + real encryption
// This mocks out the PDS and uses a no-op encrypter
func TestControllerPrivateDataPutGet(t *testing.T) {
	mockDriver := newMockDriver(node.DriverTypeDocker)
	pm := process.NewProcessManager([]process.Driver{mockDriver})
	ctrl, err := NewController2(context.Background(), pm, nil, &mockHDB{
		schema:    state.Schema(),
		jsonState: jsonStateFromNodeState(state),
	}, nil,
		nil,
		&encrypter.NoopEncrypter{},
	)
	require.NoError(t, err)

	ctx := context.Background()

	// putRecord with encryption
	coll := "my.fake.collection"
	val := map[string]any{
		"someKey": "someVal",
	}
	out, err := ctrl.putRecord(ctx, &agnostic.RepoPutRecord_Input{
		Collection: coll,
		Record:     val,
		Repo:       "my-did",
		Rkey:       "my-rkey",
	}, true)
	require.NoError(t, err)

	resp, err := ctrl.getRecord(ctx, out.Cid, coll, "my-did", "my-rkey")
	require.NoError(t, err)
	require.Equal(t, resp.Cid, out.Cid)
	require.Equal(t, val, resp.Value)

	// putRecord no encryption
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

}
