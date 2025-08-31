package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	jose "github.com/go-jose/go-jose/v3"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"
)

// testOAuthClient creates a test OAuth client with a valid JWK
func testOAuthClient(t *testing.T) OAuthClient {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwk := jose.JSONWebKey{
		Key:   key,
		KeyID: "test-key-id",
	}

	jwkBytes, err := json.Marshal(jwk)
	require.NoError(t, err)

	client, err := NewOAuthClient("test-client", "https://test.com", "https://test.com/callback", jwkBytes)
	require.NoError(t, err)

	return client
}

// testIdentity creates a test identity with a given PDS endpoint
func testIdentity(pdsEndpoint string) *identity.Identity {
	return &identity.Identity{
		DID:    "did:plc:test123",
		Handle: "test.bsky.app",
		Services: map[string]identity.Service{
			"atproto_pds": {
				Type: "AtprotoPersonalDataServer",
				URL:  pdsEndpoint,
			},
		},
	}
}

// fakeAuthServer creates a test server that responds to OAuth discovery endpoints
func fakeAuthServer(t *testing.T, responses map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource":
			if resp, ok := responses["protected-resource"]; ok {
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Default response - use proper URL format
			authServerURL := "http://" + r.Host + "/.well-known/oauth-authorization-server"
			_ = json.NewEncoder(w).Encode(oauthProtectedResource{
				AuthorizationServers: []string{authServerURL},
			})

		case "/.well-known/oauth-authorization-server":
			if resp, ok := responses["auth-server"]; ok {
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Default response - use proper URL format
			_ = json.NewEncoder(w).Encode(oauthAuthorizationServer{
				Issuer:        "https://example.com",
				TokenEndpoint: "http://" + r.Host + "/token",
				PAREndpoint:   "http://" + r.Host + "/par",
				AuthEndpoint:  "http://" + r.Host + "/auth",
			})

		case "/par":
			if resp, ok := responses["par"]; ok {
				if status, ok := responses["par-status"]; ok {
					w.WriteHeader(status.(int))
				} else {
					w.WriteHeader(http.StatusCreated)
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Default response
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(parResponse{RequestUri: "urn:ietf:params:oauth:request_uri:test"})

		case "/token":
			if status, ok := responses["token-status"]; ok {
				w.WriteHeader(status.(int))
			}

			if errorMsg, ok := responses["token-error"]; ok {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": errorMsg.(string)})
				return
			}

			if resp, ok := responses["token"]; ok {
				if str, ok := resp.(string); ok {
					_, _ = w.Write([]byte(str))
				} else {
					_ = json.NewEncoder(w).Encode(resp)
				}
				return
			}

			// Default success response
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken:  "default-access-token",
				RefreshToken: "default-refresh-token",
				Scope:        "atproto transition:generic",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
			return

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

// fakeTokenServer creates a test server that responds to token exchange endpoints
func fakeTokenServer(t *testing.T, responses map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if status, ok := responses["token-status"]; ok {
				w.WriteHeader(status.(int))
			}

			if errorMsg, ok := responses["token-error"]; ok {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": errorMsg.(string)})
				return
			}

			if resp, ok := responses["token"]; ok {
				if str, ok := resp.(string); ok {
					_, _ = w.Write([]byte(str))
				} else {
					_ = json.NewEncoder(w).Encode(resp)
				}
				return
			}

			// Default success response
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken:  "default-access-token",
				RefreshToken: "default-refresh-token",
				Scope:        "atproto transition:generic",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
}

// testDpopClient creates a real DPoP HTTP client for testing
func testDpopClient(t *testing.T, identity *identity.Identity) *DpopHttpClient {
	// Create a session store and session for testing
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Create a test request for session creation
	req := httptest.NewRequest("GET", "/test", nil)

	// Create a fresh DPoP session
	dpopSession, err := newCookieSession(req, sessionStore, identity, "https://test.com")
	require.NoError(t, err)

	// Get the key from the session
	key, exists, err := dpopSession.GetDpopKey()
	require.NoError(t, err)
	require.True(t, exists)

	// Create real DPoP client with auth server JWK builder
	return NewDpopHttpClient(key, dpopSession)
}

// testDpopClientFromSession creates a real DPoP HTTP client from an existing session
func testDpopClientFromSession(t *testing.T, dpopSession *cookieSession) *DpopHttpClient {
	// Get the key from the session
	key, exists, err := dpopSession.GetDpopKey()
	require.NoError(t, err)
	require.True(t, exists)

	// Create real DPoP client with auth server JWK builder
	return NewDpopHttpClient(key, dpopSession)
}

// DpopSessionOptions configures the behavior of testDpopSession
type DpopSessionOptions struct {
	PdsURL         string
	Issuer         *string            // nil = omit
	Identity       *identity.Identity // nil = omit
	RemoveIdentity bool               // if true, explicitly remove identity from session
	TokenInfo      *TokenResponse     // nil = omit
}

// testDpopSession creates a test DPoP session with configurable options
func testDpopSession(t *testing.T, opts DpopSessionOptions) *cookieSession {
	sessionStore := sessions.NewCookieStore([]byte("test-key"))
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Use provided identity or create a default one
	var identity *identity.Identity
	if opts.Identity != nil {
		identity = opts.Identity
	} else {
		identity = testIdentity(opts.PdsURL)
	}

	dpopSession, err := newCookieSession(req, sessionStore, identity, opts.PdsURL)
	require.NoError(t, err)
	require.NotNil(t, dpopSession)

	// Set issuer if provided
	if opts.Issuer != nil {
		err = dpopSession.SetIssuer(*opts.Issuer)
		require.NoError(t, err)
	}

	// Set access token if provided
	if opts.TokenInfo != nil {
		err = dpopSession.SetTokenInfo(opts.TokenInfo)
		require.NoError(t, err)
	}

	// Remove identity if explicitly requested
	if opts.RemoveIdentity {
		delete(dpopSession.session.Values, cIdentitySessionKey)
	}

	// Save the session
	dpopSession.Save(req, w)

	return dpopSession
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
