package privi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/eagraf/habitat-new/internal/permissions"
	"github.com/stretchr/testify/require"
)

// A unit test testing putRecord and getRecord with one basic permission.
// TODO: an integration test with two PDS's + privi servers running.
func TestControllerPrivateDataPutGet(t *testing.T) {
	ctx := context.Background()
	// The val the caller is trying to put
	val := map[string]any{
		"someKey": "someVal",
	}
	marshalledVal, err := json.Marshal(val)
	require.NoError(t, err)

	dummy := permissions.NewDummyStore()
	p := newStore(syntax.DID("my-did"), make(inMemoryRepo), dummy)

	// putRecord
	coll := "my.fake.collection"
	rkey := "my-rkey"
	validate := true
	err = p.putRecord(ctx, coll, val, rkey, &validate)
	require.NoError(t, err)

	got, err := p.getRecord(ctx, coll, "my-rkey", "another-did")
	require.Nil(t, got)
	require.ErrorIs(t, ErrUnauthorized, err)

	require.NoError(t, dummy.AddLexiconReadPermission(coll, "another-did"))

	got, err = p.getRecord(ctx, coll, "my-rkey", "another-did")
	require.NoError(t, err)
	require.Equal(t, []byte(got), marshalledVal)

	err = p.putRecord(ctx, coll, val, rkey, &validate)
	require.NoError(t, err)
}
