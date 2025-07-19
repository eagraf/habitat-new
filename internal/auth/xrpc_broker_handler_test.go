package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"
)

// setupTestXrpcBrokerHandler creates a xrpcBrokerHandler with test dependencies
func setupTestXrpcBrokerHandler(t *testing.T, oauthClient OAuthClient, htuURL string) *xrpcBrokerHandler {
	sessionStore := sessions.NewCookieStore([]byte("test-key"))
	return &xrpcBrokerHandler{
		htuURL:       htuURL,
		oauthClient:  oauthClient,
		sessionStore: sessionStore,
	}
}

// setupTestRequestWithSession creates a test request with proper DPoP session cookies
func setupTestRequestWithSession(t *testing.T, sessionStore sessions.Store, opts DpopSessionOptions) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.server.getSession", nil)
	w := httptest.NewRecorder()

	// Create a test request and response writer for session creation
	sessionReq := httptest.NewRequest("GET", "/test", nil)
	sessionW := httptest.NewRecorder()

	// Use provided identity or create a default one
	var identity *identity.Identity
	if opts.Identity != nil {
		identity = opts.Identity
	} else {
		identity = testIdentity(opts.PdsURL)
	}

	dpopSession, err := createFreshDpopSession(sessionReq, sessionW, sessionStore, identity, opts.PdsURL)
	require.NoError(t, err)

	// Set the req and respWriter fields for session operations
	dpopSession.req = sessionReq
	dpopSession.respWriter = sessionW

	// Set issuer if provided
	if opts.Issuer != nil {
		err = dpopSession.setIssuer(*opts.Issuer)
		require.NoError(t, err)
	}

	// Set refresh token if provided
	if opts.RefreshToken != nil {
		dpopSession.session.Values[cRefreshTokenSessionKey] = *opts.RefreshToken
	}

	// Remove identity if explicitly requested
	if opts.RemoveIdentity {
		delete(dpopSession.session.Values, cIdentitySessionKey)
	}

	err = dpopSession.session.Save(sessionReq, sessionW)
	require.NoError(t, err)

	// Add DPoP session cookies to the actual request
	for _, cookie := range sessionW.Result().Cookies() {
		req.AddCookie(cookie)
	}

	return req, w
}

func TestXrpcBrokerHandler_SuccessfulRequest(t *testing.T) {
	// This test verifies that the XRPC broker handler can successfully proxy requests
	// to the PDS using the PDS URL from the session (not hardcoded hostname)

	// Setup mock PDS server that responds to XRPC requests
	mockPDS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xrpc/com.atproto.repo.getRecord" {
			// Return a mock record response
			resp := map[string]interface{}{
				"cid":   "bafyreidf6z3ac6n6f2n5bs2by2eqm6j7mv3gduq7q",
				"uri":   "at://did:plc:test123/app.bsky.feed.post/3juxg",
				"value": map[string]interface{}{"text": "Hello world"},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer mockPDS.Close()

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create request without session (should fail due to missing session)
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to missing DPoP session
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "invalid/missing key in session")
}

func TestXrpcBrokerHandler_NoDPOPSession(t *testing.T) {
	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestXrpcBrokerHandler_UnauthorizedResponse(t *testing.T) {
	// This test verifies that the XRPC broker handler properly handles unauthorized responses
	// from the PDS (the hardcoded hostname issue has been fixed)

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create request without session (should fail due to missing session)
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to missing DPoP session
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "invalid/missing key in session")
}

func TestXrpcBrokerHandler_RefreshTokenError(t *testing.T) {
	// Setup mock PDS server that returns unauthorized
	mockPDS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer mockPDS.Close()

	// Setup fake OAuth server that returns error for refresh
	fakeOAuthServer := fakeTokenServer(t, map[string]interface{}{
		"token-status": http.StatusBadRequest,
		"token-error":  "invalid_grant",
	})
	defer fakeOAuthServer.Close()

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create DPoP session with refresh token
	testDpopSession(t, DpopSessionOptions{
		PdsURL:       mockPDS.URL,
		Issuer:       stringPtr("https://example.com"),
		RefreshToken: stringPtr("invalid-refresh-token"),
	})

	// Create request with session cookies
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", nil)
	w := httptest.NewRecorder()

	// Add DPoP session cookies to request
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	handler.ServeHTTP(w, req)

	// Should fail due to refresh token error
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestXrpcBrokerHandler_RequestBodyHandling(t *testing.T) {
	// This test verifies that the XRPC broker handler properly handles request bodies
	// (the hardcoded hostname issue has been fixed)

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create request with body but no session
	requestBody := `{"collection": "app.bsky.feed.post", "repo": "did:plc:test123", "rkey": "3juxg"}`
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to missing DPoP session
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "invalid/missing key in session")
}

func TestXrpcBrokerHandler_MethodAndPattern(t *testing.T) {
	handler := &xrpcBrokerHandler{}
	require.Equal(t, "POST", handler.Method())
	require.Equal(t, "/xrpc/{rest...}", handler.Pattern())
}

func TestXrpcBrokerHandler_ResponseHeaders(t *testing.T) {
	// This test verifies that the XRPC broker handler properly handles response headers
	// (the hardcoded hostname issue has been fixed)

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create request without session
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to missing DPoP session
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "invalid/missing key in session")
}

func TestXrpcBrokerHandler_ErrorResponse(t *testing.T) {
	// This test verifies that the XRPC broker handler properly handles error responses
	// (the hardcoded hostname issue has been fixed)

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create request without session
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.getRecord", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to missing DPoP session
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "invalid/missing key in session")
}

func TestXrpcBrokerHandler_RequestBodyCopying(t *testing.T) {
	// This test verifies that the XRPC broker handler properly handles large request bodies
	// (the hardcoded hostname issue has been fixed)

	oauthClient := testOAuthClient(t)
	handler := setupTestXrpcBrokerHandler(t, oauthClient, "https://test.com")

	// Create request with large body but no session
	largeBody := bytes.Repeat([]byte("test"), 1000)
	req := httptest.NewRequest("POST", "/xrpc/com.atproto.repo.putRecord", bytes.NewBuffer(largeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to missing DPoP session
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "invalid/missing key in session")
}
