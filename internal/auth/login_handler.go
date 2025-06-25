package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	jose "github.com/go-jose/go-jose/v3"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
)

type loginHandler struct {
	oauthClient  OAuthClient
	sessionStore sessions.Store

	// Optional. If set, the DID will be resolved by calling
	// com.atproto.identity.resolveHandle on the PDS at this URL.
	pdsURL string
}

type metadataHandler struct {
	oauthClient OAuthClient
}

type callbackHandler struct {
	oauthClient  OAuthClient
	sessionStore sessions.Store
}

func NewLoginHandler(
	nodeConfig *config.NodeConfig,
	sessionStore sessions.Store,
) (login api.Route, metadata api.Route, callback api.Route) {
	key, err := ecdsa.GenerateKey(
		elliptic.P256(),
		bytes.NewReader(bytes.Repeat([]byte("hello world"), 1024)),
	)
	if err != nil {
		log.Error().Err(err).Msg("error generating key")
		panic(err)
	}
	jwk, err := json.Marshal(jose.JSONWebKey{
		Key:       key,
		KeyID:     "habitat",
		Algorithm: string(jose.ES256),
		Use:       "sig",
	})
	if err != nil {
		log.Error().Err(err).Msg("error marshalling jwk")
		panic(err)
	}
	oauthClient, err := NewOAuthClient(
		"https://beacon-dev.tail07d32.ts.net/habitat/api/client-metadata.json", /*clientId*/
		"https://beacon-dev.tail07d32.ts.net/habitat/api/auth-callback",        /*redirectUri*/
		jwk, /*secretJwk*/
	)
	if err != nil {
		log.Error().Err(err).Msg("error creating oauth client")
		panic(err)
	}

	return &loginHandler{
			oauthClient:  oauthClient,
			sessionStore: sessionStore,
			pdsURL:       nodeConfig.InternalPDSURL(),
		}, &metadataHandler{
			oauthClient: oauthClient,
		}, &callbackHandler{
			oauthClient:  oauthClient,
			sessionStore: sessionStore,
		}
}

// Method implements api.Route.
func (l *loginHandler) Method() string {
	return http.MethodGet
}

// Pattern implements api.Route.
func (l *loginHandler) Pattern() string {
	return "/login"
}

// ServeHTTP implements api.Route.
func (l *loginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	identifier := r.Form.Get("handle")

	atid, err := syntax.ParseAtIdentifier(identifier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var loginHint *string
	var id *identity.Identity
	if l.pdsURL != "" {
		client := &xrpc.Client{
			Host: l.pdsURL,
		}
		handle := atid.String()
		loginHint = &handle
		resp, err := atproto.IdentityResolveHandle(r.Context(), client, handle)
		if err != nil {
			log.Warn().Err(err).Str("identifier", identifier).Msg("error resolving handle")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		did, err := syntax.ParseDID(resp.Did)
		if err != nil {
			log.Warn().Err(err).Str("did", resp.Did).Msg("error parsing did")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id = &identity.Identity{
			DID:    did,
			Handle: syntax.Handle(handle),
			Services: map[string]identity.Service{
				"atproto_pds": {
					URL: l.pdsURL,
				},
			},
		}
	} else {
		loginHint = nil
		id, err = identity.DefaultDirectory().Lookup(r.Context(), *atid)
		if err != nil {
			log.Warn().Err(err).Str("identifier", identifier).Msg("error looking up identifier")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	dpopSession, _ := l.sessionStore.New(r, "dpop-session")
	dpopClient := NewDpopHttpClient(dpopSession)
	dpopKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dpopClient.SetKey(dpopKey)
	dpopClient.SetIdentity(id)
	dpopClient.SetPDSURL(l.pdsURL)

	redirect, state, err := l.oauthClient.Authorize(dpopClient, id, loginHint)
	if err != nil {
		log.Error().Err(err).Str("identifier", identifier).Msg("error authorizing user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = dpopSession.Save(r, w)
	if err != nil {
		log.Error().Err(err).Msg("error saving dpop session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stateJson, err := json.Marshal(state)
	if err != nil {
		log.Error().Err(err).Str("identifier", identifier).Msg("error marshalling state")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session, _ := l.sessionStore.New(r, "auth-session")
	session.AddFlash(stateJson)
	err = session.Save(r, w)
	if err != nil {
		log.Error().Err(err).Str("identifier", identifier).Msg("error saving session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// Method implements api.Route.
func (m *metadataHandler) Method() string {
	return http.MethodGet
}

// Pattern implements api.Route.
func (m *metadataHandler) Pattern() string {
	return "/client-metadata.json"
}

// ServeHTTP implements api.Route.
func (m *metadataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.Marshal(m.oauthClient.ClientMetadata())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(bytes)
}

// Method implements api.Route.
func (c *callbackHandler) Method() string {
	return http.MethodGet
}

// Pattern implements api.Route.
func (c *callbackHandler) Pattern() string {
	return "/auth-callback"
}

// ServeHTTP implements api.Route.
func (c *callbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info().Msgf("query: %s", r.Cookies())
	session, err := c.sessionStore.Get(r, "auth-session")
	if err != nil {
		log.Error().Err(err).Msg("error getting session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	flashes := session.Flashes()
	session.Save(r, w)
	if len(flashes) == 0 {
		http.Error(w, "no state in session", http.StatusBadRequest)
		return
	}
	stateJson, ok := flashes[0].([]byte)
	if !ok {
		http.Error(w, "invalid state in session", http.StatusBadRequest)
		return
	}

	var state AuthorizeState
	err = json.Unmarshal(stateJson, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	code := r.URL.Query().Get("code")
	issuer := r.URL.Query().Get("iss")

	dpopSession, err := c.sessionStore.Get(r, "dpop-session")
	if err != nil {
		log.Error().Err(err).Msg("error getting dpop session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dpopClient := NewDpopHttpClient(dpopSession)
	tokenResp, err := c.oauthClient.ExchangeCode(dpopClient, code, issuer, &state)
	if err != nil {
		log.Error().Err(err).Str("code", code).Str("issuer", issuer).Msg("error exchanging code")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dpopClient.SetAccessTokenHash(tokenResp.AccessToken)
	dpopClient.SetAccessToken(tokenResp.AccessToken)
	dpopClient.SetRefreshToken(tokenResp.RefreshToken)
	dpopClient.SetIssuer(issuer)

	err = dpopSession.Save(r, w)
	if err != nil {
		log.Error().Err(err).Msg("error saving dpop session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// This is temporary to make it work with the frontend
	http.SetCookie(w, &http.Cookie{
		Name:  "access_token",
		Value: tokenResp.AccessToken,
		Path:  "/",
		//		HttpOnly: true,
		//Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	if tokenResp.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:  "refresh_token",
			Value: tokenResp.RefreshToken,
			Path:  "/",
			//HttpOnly: true,
			//Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(
		w,
		r,
		"/",
		http.StatusSeeOther,
	)
}
