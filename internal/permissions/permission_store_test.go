package permissions

import (
	"os"
	"testing"

	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"github.com/stretchr/testify/require"
)

func TestPermissionStore_TestPolicy1(t *testing.T) {
	a := fileadapter.NewAdapter("test_policies/test_policy_1.csv")
	ps, err := NewStore(a, false)
	require.NoError(t, err)

	// inheritance
	ok, err := ps.HasPermission("did:1", "app.bsky.posts", "post1", Read)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = ps.HasPermission("did:1", "app.bsky.posts", "post2", Read)
	require.NoError(t, err)
	require.False(t, ok)

	// glob match on record key
	ok, err = ps.HasPermission("did:1", "app.bsky.likes", "like1", Write)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = ps.HasPermission("did:1", "app.bsky.likes", "like2", Write)
	require.NoError(t, err)
	require.True(t, ok)

	// glob match on nsid
	ok, err = ps.HasPermission("did:2", "app.bsky.videos", "video1", Write)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = ps.HasPermission("did:2", "app.bsky.photos", "photo1", Write)
	require.NoError(t, err)
	require.True(t, ok)

	// action match
	ok, err = ps.HasPermission("did:1", "app.bsky.likes", "like1", Read)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestPermissionStoreAddRemovePolicies(t *testing.T) {
	inner, err := os.ReadFile("test_policies/test_add_remove_policies.csv")
	require.NoError(t, err)
	tmp, err := os.CreateTemp("test_policies", "test-tmp")
	require.NoError(t, err)
	// defer os.Remove(tmp.Name())
	_, err = tmp.Write(inner)
	require.NoError(t, err)

	a := fileadapter.NewAdapter(tmp.Name())
	ps, err := NewStore(a, true)
	require.NoError(t, err)

	// inheritance
	ok, err := ps.HasPermission("did:1", "app.bsky.likes", "like1", Read)
	require.NoError(t, err)
	require.True(t, ok)

	prev, err := os.ReadFile(tmp.Name())
	require.NoError(t, err)
	err = ps.AddLexiconReadPermission("did:1", "app.bsky.likes-new")
	require.NoError(t, err)

	ok, err = ps.HasPermission("did:1", "app.bsky.likes_new", "mynewlike", Read)
	require.NoError(t, err)
	require.True(t, ok)
	// TODO: check exact change
	next, err := os.ReadFile(tmp.Name())
	require.NoError(t, err)
	require.NotEqual(t, prev, next)
}
