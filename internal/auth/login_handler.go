package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
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
	oauthClient       OAuthClient
	sessionStore      sessions.Store
	habitatNodeDomain string

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

func GetRoutes(
	nodeConfig *config.NodeConfig,
	sessionStore sessions.Store,
) ([]api.Route, error) {
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

	baseClientURL := nodeConfig.ExternalURL()
	oauthClient, err := NewOAuthClient(
		baseClientURL+"/habitat/api/client-metadata.json", /*clientId*/
		baseClientURL, /*clientUri*/
		baseClientURL+"/habitat/api/auth-callback", /*redirectUri*/
		jwk, /*secretJwk*/
	)
	if err != nil {
		log.Error().Err(err).Msg("error creating oauth client")
		return nil, err
	}

	return []api.Route{
		&loginHandler{
			pdsURL:            nodeConfig.InternalPDSURL(),
			oauthClient:       oauthClient,
			sessionStore:      sessionStore,
			habitatNodeDomain: nodeConfig.Domain(),
		}, &metadataHandler{
			oauthClient: oauthClient,
		}, &callbackHandler{
			oauthClient:  oauthClient,
			sessionStore: sessionStore,
		},
	}, nil
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

	handle := r.Form.Get("handle")

	atid, err := syntax.ParseAtIdentifier(handle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, loginHint, err := l.resolveIdentity(atid, handle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dpopSession, err := newHabitatGorillaSession(r, w, l.sessionStore, id, l.pdsURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the key from the session
	key, err := dpopSession.GetDpopKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create nonce provider from session
	nonceProvider := NewSessionNonceProvider(dpopSession)

	dpopClient := NewDpopHttpClient(key, nonceProvider)

	redirect, state, err := l.oauthClient.Authorize(dpopClient, id, loginHint)
	if err != nil {
		log.Error().Err(err).Str("identifier", handle).Msg("error authorizing user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stateJson, err := json.Marshal(state)
	if err != nil {
		log.Error().Err(err).Str("identifier", handle).Msg("error marshalling state")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session, _ := l.sessionStore.New(r, "auth-session")
	session.AddFlash(stateJson)

	dpopSession.Save(r, w)
	err = session.Save(r, w)
	if err != nil {
		log.Error().Err(err).Msg("error saving session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (l *loginHandler) resolveIdentity(atID *syntax.AtIdentifier, handle string) (*identity.Identity, *string, error) {
	pdsOnHabitat := doesHandleBelongToDomain(handle, l.habitatNodeDomain)
	if pdsOnHabitat {
		client := &xrpc.Client{
			Host: l.pdsURL,
		}
		handle := atID
		loginHint := &handle
		resp, err := atproto.IdentityResolveHandle(r.Context(), client, handle)
		if err != nil {
			return nil, nil, err
		}
		did, err := syntax.ParseDID(resp.Did)
		if err != nil {
			return nil, nil, err
		}
		return &identity.Identity{
			DID:    did,
			Handle: syntax.Handle(handle),
			Services: map[string]identity.Service{
				"atproto_pds": {
					URL: l.pdsURL,
				},
			},
		}, loginHint, nil
	} else {
		id, err := identity.DefaultDirectory().Lookup(r.Context(), *atid)
		if err != nil {
			return nil, nil, err
		}
		return id, nil, nil
	}

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
	_, err = w.Write(bytes)
	if err != nil {
		log.Error().Err(err).Msg("error writing response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	authSession, err := c.sessionStore.Get(r, "auth-session")
	if err != nil {
		log.Error().Err(err).Msg("error getting session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	flashes := authSession.Flashes()
	err = authSession.Save(r, w)
	if err != nil {
		log.Error().Err(err).Msg("error saving auth session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

	dpopSession, err := getExistingHabitatGorillaSession(r, w, c.sessionStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dpopSession.Save(r, w)

	// Get the key from the session
	key, err := dpopSession.GetDpopKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create nonce provider from session
	nonceProvider := NewSessionNonceProvider(dpopSession)

	dpopClient := NewDpopHttpClient(key, nonceProvider)

	tokenResp, err := c.oauthClient.ExchangeCode(dpopClient, code, issuer, &state)
	if err != nil {
		log.Error().Err(err).Str("code", code).Str("issuer", issuer).Msg("error exchanging code")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = dpopSession.SetTokenInfo(tokenResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = dpopSession.SetIssuer(issuer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	identity, err := dpopSession.GetIdentity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "handle",
		Value:    string(identity.Handle),
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "did",
		Value:    string(identity.DID),
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	pdsURL, err := dpopSession.GetPDSURL()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pdsURL != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "pds_url",
			Value:    pdsURL,
			Path:     "/",
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
