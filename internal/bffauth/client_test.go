package bffauth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/eagraf/habitat-new/internal/resolvers"
	"github.com/stretchr/testify/require"
)

func FakeHabitatHostResolver(habitatHost string) resolvers.HabitatHostResolver {
	return func(did string) (string, error) {
		// Map of test DIDs to habitat hosts
		testHosts := map[string]string{
			"did:test:123": habitatHost,
			"did:test:456": "other-host.example.com",
			"did:test:789": "another-host.example.com",
		}

		host, ok := testHosts[did]
		if !ok {
			return "", fmt.Errorf("no habitat host found for did: %s", did)
		}
		return host, nil
	}
}

func FakePublicKeyResolver(publicKey crypto.PublicKey) resolvers.PublicKeyResolver {
	return func(did string) (crypto.PublicKey, error) {
		// Map of test DIDs to public keys
		testKeys := map[string]crypto.PublicKey{
			"did:test:123": publicKey,
			"did:test:456": publicKey,
			"did:test:789": publicKey,
		}

		key, ok := testKeys[did]
		if !ok {
			return nil, fmt.Errorf("no public key found for did: %s", did)
		}
		return key, nil
	}
}

func TestClient(t *testing.T) {
	// Create test server with provider endpoints
	signingKey := []byte("test-signing-key")
	provider := NewProvider(NewInMemorySessionPersister(), signingKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/habitat/api/node/bff/challenge":
			provider.handleChallenge(w, r)
		case "/habitat/api/node/bff/auth":
			provider.handleAuth(w, r)
		case "/habitat/api/node/bff/test":
			provider.handleTest(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Generate test key and user
	testKey, err := crypto.GeneratePrivateKeyP256()
	require.NoError(t, err)

	testPubKey, err := testKey.PublicKey()
	require.NoError(t, err)

	testUser := &ExternalHabitatUser{
		DID:       "did:test:123",
		PublicKey: testPubKey,
		Host:      server.URL[7:], // Strip http:// prefix
	}

	// Create client with test data
	c := &client{
		privateKey:          testKey,
		activeTokens:        make(map[string]string),
		scheme:              "http",
		habitatHostResolver: FakeHabitatHostResolver(server.URL[7:]),
		publicKeyResolver:   FakePublicKeyResolver(testPubKey),
	}

	// Test getting token
	token, err := c.GetToken(testUser.DID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Test hitting the test endpoint
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", server.URL+"/habitat/api/node/bff/test", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Test token caching
	cachedToken, err := c.GetToken(testUser.DID)
	require.NoError(t, err)
	require.Equal(t, token, cachedToken)

	// Test error case - invalid DID
	_, err = c.GetToken("did:test:invalid")
	require.Error(t, err)
}
