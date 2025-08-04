package auth

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/gorilla/sessions"
)

type xrpcBrokerHandler struct {
	htuURL       string
	oauthClient  OAuthClient
	sessionStore sessions.Store
}

func (h *xrpcBrokerHandler) Method() string {
	return http.MethodPost
}

func (h *xrpcBrokerHandler) Pattern() string {
	return "/xrpc/{rest...}"
}

func (h *xrpcBrokerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Note: this is effectively acting as a reverse proxy in front of the XRPC endpoint.
	// Using the main Habitat reverse proxy isn't sufficient because of the additional
	// roundtrips DPoP requires.
	dpopSession, err := getCookieSession(r, w, h.sessionStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dpopSession.Save(r, w)

	htu := path.Join(h.htuURL, r.URL.Path)

	// Get the key from the session
	key, err := dpopSession.GetDpopKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get PDS URL from session
	pdsURL, err := dpopSession.GetPDSURL()
	fmt.Println("PDS URL", pdsURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse PDS URL to get host and scheme
	parsedPDSURL, err := url.Parse(pdsURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Unset the request URI (can't be set when used as a client)
	r.RequestURI = ""
	r.URL.Scheme = parsedPDSURL.Scheme
	r.URL.Host = parsedPDSURL.Host
	tokenInfo, err := dpopSession.GetTokenInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	accessToken := tokenInfo.AccessToken
	//r.Header.Set("Authorization", "DPoP "+accessToken)
	r.Header.Del("Content-Length")

	pdsDpopClient := NewDpopHttpClient(key, dpopSession, WithHTU(htu), WithAccessToken(accessToken))

	// Copy the request body without consuming it
	bodyCopy, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))

	// Forward the request to the PDS
	resp, err := pdsDpopClient.Do(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check if we need to refresh the token
	if resp.StatusCode == http.StatusUnauthorized {
		// Try to refresh the token
		authDpopClient := NewDpopHttpClient(key, dpopSession)
		identity, err := dpopSession.GetIdentity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		issuer, err := dpopSession.GetIssuer()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tokenResp, err := h.oauthClient.RefreshToken(authDpopClient, identity, issuer, tokenInfo.RefreshToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = dpopSession.SetTokenInfo(tokenResp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))

		refreshedDpopClient := NewDpopHttpClient(key, dpopSession, WithHTU(htu), WithAccessToken(tokenResp.AccessToken))

		// Retry the request with the new token
		resp, err = refreshedDpopClient.Do(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
	}

	// Writing out the response as we got it.

	// Copy response headers before writing status code
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Write body last
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	if err == http.ErrBodyReadAfterClose {
		return
	}
}
