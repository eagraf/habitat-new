package auth

import (
	"bytes"
	"errors"
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
	dpopSession, err := getCookieSession(r, h.sessionStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dpopSession.Save(r, w)

	pdsDpopClient, err := h.getForwardingDpopClient(r, dpopSession)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	forwardReq, err := h.getForwardReq(r, dpopSession)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy the request body without consuming it
	bodyCopy, err := io.ReadAll(forwardReq.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	forwardReq.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))

	// Forward the request to the PDS
	resp, err := pdsDpopClient.Do(forwardReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check if we need to refresh the token
	if resp.StatusCode == http.StatusUnauthorized {
		err = h.refreshSession(dpopSession)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		forwardReq, err = h.getForwardReq(r, dpopSession)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		forwardReq.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))

		refreshedDpopClient, err := h.getForwardingDpopClient(r, dpopSession)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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

func (h *xrpcBrokerHandler) getForwardReq(r *http.Request, dpopSession *cookieSession) (*http.Request, error) {
	// Clone the initial request. Note this only makes a shallow copy of the body
	newReq := r.Clone(r.Context())

	// Get PDS URL from session
	pdsURL, ok, err := dpopSession.GetPDSURL()
	if !ok || err != nil {
		return nil, errors.New("no pds url found in session")
	}

	// Parse PDS URL to get host and scheme
	parsedPDSURL, err := url.Parse(pdsURL)
	if err != nil {
		return nil, err
	}

	// Unset the request URI (can't be set when used as a client)
	newReq.RequestURI = ""
	newReq.URL.Scheme = parsedPDSURL.Scheme
	newReq.URL.Host = parsedPDSURL.Host
	newReq.Header.Del("Content-Length")

	return newReq, nil
}

func (h *xrpcBrokerHandler) getForwardingDpopClient(r *http.Request, dpopSession *cookieSession) (*DpopHttpClient, error) {
	htu := path.Join(h.htuURL, r.URL.Path)

	// Get the key from the session
	key, ok, err := dpopSession.GetDpopKey()
	if !ok || err != nil {
		return nil, errors.New("no key in session")
	}

	tokenInfo, ok, err := dpopSession.GetTokenInfo()
	if !ok || err != nil {
		return nil, errors.New("no token info in session")
	}
	accessToken := tokenInfo.AccessToken

	return NewDpopHttpClient(key, dpopSession, WithHTU(htu), WithAccessToken(accessToken)), nil
}

func (h *xrpcBrokerHandler) refreshSession(dpopSession *cookieSession) error {
	key, ok, err := dpopSession.GetDpopKey()
	if !ok || err != nil {
		return errors.New("no key in session")
	}

	authDpopClient := NewDpopHttpClient(key, dpopSession)
	identity, ok, err := dpopSession.GetIdentity()
	if !ok || err != nil {
		return errors.New("no identity in session")
	}

	issuer, ok, err := dpopSession.GetIssuer()
	if !ok || err != nil {
		return errors.New("no issuer in session")
	}

	tokenInfo, ok, err := dpopSession.GetTokenInfo()
	if !ok || err != nil {
		return errors.New("no token info in session")
	}

	tokenResp, err := h.oauthClient.RefreshToken(authDpopClient, identity, issuer, tokenInfo.RefreshToken)
	if err != nil {
		return err
	}
	err = dpopSession.SetTokenInfo(tokenResp)
	if err != nil {
		return err
	}

	return nil
}
