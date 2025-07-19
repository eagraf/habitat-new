package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
				json.NewEncoder(w).Encode(resp)
				return
			}
			// Default response - use proper URL format
			authServerURL := "http://" + r.Host + "/.well-known/oauth-authorization-server"
			json.NewEncoder(w).Encode(oauthProtectedResource{
				AuthorizationServers: []string{authServerURL},
			})

		case "/.well-known/oauth-authorization-server":
			if resp, ok := responses["auth-server"]; ok {
				json.NewEncoder(w).Encode(resp)
				return
			}
			// Default response - use proper URL format
			json.NewEncoder(w).Encode(oauthAuthorizationServer{
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
				json.NewEncoder(w).Encode(resp)
				return
			}
			// Default response
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(parResponse{RequestUri: "urn:ietf:params:oauth:request_uri:test"})

		case "/token":
			if status, ok := responses["token-status"]; ok {
				w.WriteHeader(status.(int))
			}

			if errorMsg, ok := responses["token-error"]; ok {
				json.NewEncoder(w).Encode(map[string]string{"error": errorMsg.(string)})
				return
			}

			if resp, ok := responses["token"]; ok {
				if str, ok := resp.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(resp)
				}
				return
			}

			// Default success response
			json.NewEncoder(w).Encode(TokenResponse{
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
				json.NewEncoder(w).Encode(map[string]string{"error": errorMsg.(string)})
				return
			}

			if resp, ok := responses["token"]; ok {
				if str, ok := resp.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(resp)
				}
				return
			}

			// Default success response
			json.NewEncoder(w).Encode(TokenResponse{
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

	// Create a test request and response writer for session creation
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Create a fresh DPoP session
	dpopSession, err := createFreshDpopSession(req, w, sessionStore, identity, "https://test.com")
	require.NoError(t, err)

	// Create real DPoP client with auth server JWK builder
	return NewDpopHttpClient(dpopSession, getAuthServerJWKBuilder())
}

// testDpopClientFromSession creates a real DPoP HTTP client from an existing session
func testDpopClientFromSession(t *testing.T, dpopSession *dpopSession) *DpopHttpClient {
	// Create real DPoP client with auth server JWK builder
	return NewDpopHttpClient(dpopSession, getAuthServerJWKBuilder())
}

// DpopSessionOptions configures the behavior of testDpopSession
type DpopSessionOptions struct {
	PdsURL         string
	Issuer         *string            // nil = omit
	AccessToken    *string            // nil = omit
	RefreshToken   *string            // nil = omit
	Identity       *identity.Identity // nil = omit
	RemoveIdentity bool               // if true, explicitly remove identity from session
}

// testDpopSession creates a test DPoP session with configurable options
func testDpopSession(t *testing.T, opts DpopSessionOptions) *dpopSession {
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

	dpopSession, err := createFreshDpopSession(req, w, sessionStore, identity, opts.PdsURL)
	require.NoError(t, err)

	// Set the req and respWriter fields for session operations
	dpopSession.req = req
	dpopSession.respWriter = w

	// Set issuer if provided
	if opts.Issuer != nil {
		err = dpopSession.setIssuer(*opts.Issuer)
		require.NoError(t, err)
	}

	// Set access token if provided
	if opts.AccessToken != nil {
		dpopSession.session.Values[cAccessTokenSessionKey] = *opts.AccessToken

		// Set access token hash (required for DPoP)
		h := sha256.New()
		h.Write([]byte(*opts.AccessToken))
		hash := h.Sum(nil)
		encodedHash := base64.RawURLEncoding.EncodeToString(hash)
		dpopSession.session.Values[cAccessTokenHashSessionKey] = encodedHash
	}

	// Set refresh token if provided
	if opts.RefreshToken != nil {
		dpopSession.session.Values[cRefreshTokenSessionKey] = *opts.RefreshToken
	}

	// Remove identity if explicitly requested
	if opts.RemoveIdentity {
		delete(dpopSession.session.Values, cIdentitySessionKey)
	}

	err = dpopSession.session.Save(req, w)
	require.NoError(t, err)

	return dpopSession
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
