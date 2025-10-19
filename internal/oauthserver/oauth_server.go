package oauthserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/eagraf/habitat-new/internal/auth"
	"github.com/gorilla/sessions"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
)

const (
	sessionName = "auth-session"
)

type authRequestFlash struct {
	ClientID       string
	RedirectURI    *url.URL
	State          string
	Scopes         []string
	ResponseTypes  []string
	Form           url.Values
	Session        *session
	AuthorizeState *auth.AuthorizeState
}

type OAuthServer struct {
	storage      *store
	provider     fosite.OAuth2Provider
	sessionStore sessions.Store
	oauthClient  auth.OAuthClient
	directory    identity.Directory
}

func NewOAuthServer(
	oauthClient auth.OAuthClient,
	sessionStore sessions.Store,
	directory identity.Directory,
) *OAuthServer {
	storage := newStore()
	config := &fosite.Config{
		GlobalSecret:               []byte("my super secret signing password"),
		SendDebugMessagesToClients: true,
	}
	provider := compose.Compose(
		config,
		storage,
		compose.NewOAuth2HMACStrategy(config),
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2PKCEFactory,
	)
	gob.Register(&authRequestFlash{})
	gob.Register(auth.AuthorizeState{})
	return &OAuthServer{
		storage:      storage,
		provider:     provider,
		oauthClient:  oauthClient,
		sessionStore: sessionStore,
		directory:    directory,
	}
}

func (o *OAuthServer) HandleAuthorize(
	w http.ResponseWriter,
	r *http.Request,
) error {
	ctx := r.Context()
	requester, err := o.provider.NewAuthorizeRequest(ctx, r)
	if err != nil {
		o.provider.WriteAuthorizeError(ctx, w, requester, err)
		return nil
	}
	if r.ParseForm() != nil {
		return fmt.Errorf("failed to parse form: %w", err)
	}
	handle := r.Form.Get("handle")
	atid, err := syntax.ParseAtIdentifier(handle)
	if err != nil {
		return fmt.Errorf("failed to parse handle: %w", err)
	}
	id, err := o.directory.Lookup(ctx, *atid)
	if err != nil {
		return fmt.Errorf("failed to lookup identity: %w", err)
	}
	dpopKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}
	dpopClient := auth.NewDpopHttpClient(dpopKey, &nonceProvider{})
	redirect, state, err := o.oauthClient.Authorize(dpopClient, id)
	if err != nil {
		return fmt.Errorf("failed to authorize: %w", err)
	}
	authorizeSession, _ := o.sessionStore.New(r, sessionName)
	authorizeSession.AddFlash(&authRequestFlash{
		ClientID:       requester.GetClient().GetID(),
		RedirectURI:    requester.GetRedirectURI(),
		State:          requester.GetState(),
		Scopes:         requester.GetRequestedScopes(),
		ResponseTypes:  requester.GetResponseTypes(),
		Form:           requester.GetRequestForm(),
		AuthorizeState: state,
		Session:        newSession(handle, dpopKey),
	})
	if err := authorizeSession.Save(r, w); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

func (o *OAuthServer) HandleCallback(
	w http.ResponseWriter,
	r *http.Request,
) error {
	ctx := r.Context()
	authorizeSession, err := o.sessionStore.Get(r, sessionName)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	flashes := authorizeSession.Flashes()
	if len(flashes) == 0 {
		return fmt.Errorf("failed to get auth request flash")
	}
	arf, ok := flashes[0].(*authRequestFlash)
	if !ok {
		return fmt.Errorf("failed to parse request flash")
	}
	authRequest := &fosite.AuthorizeRequest{
		Request: fosite.Request{
			Form: arf.Form,
			Client: &client{
				ClientMetadata: auth.ClientMetadata{
					ClientId: arf.ClientID,
				},
			},
			RequestedScope: arf.Scopes,
		},
		RedirectURI:   arf.RedirectURI,
		State:         arf.State,
		ResponseTypes: arf.ResponseTypes,
	}
	dpopClient := auth.NewDpopHttpClient(arf.Session.DpopKey, &nonceProvider{})
	tokenInfo, err := o.oauthClient.ExchangeCode(
		dpopClient,
		r.URL.Query().Get("code"),
		r.URL.Query().Get("iss"),
		arf.AuthorizeState,
	)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}
	arf.Session.SetTokenInfo(tokenInfo)
	resp, err := o.provider.NewAuthorizeResponse(ctx, authRequest, arf.Session)
	o.provider.WriteAuthorizeResponse(r.Context(), w, authRequest, resp)
	return nil
}

func (o *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var session session
	req, err := o.provider.NewAccessRequest(ctx, r, &session)
	if err != nil {
		o.provider.WriteAccessError(ctx, w, req, err)
		return
	}
	resp, err := o.provider.NewAccessResponse(ctx, req)
	if err != nil {
		o.provider.WriteAccessError(ctx, w, req, err)
		return
	}
	o.provider.WriteAccessResponse(ctx, w, req, resp)
}

type nonceProvider struct{ nonce string }

var _ auth.DpopNonceProvider = (*nonceProvider)(nil)

// GetDpopNonce implements auth.DpopNonceProvider.
func (n *nonceProvider) GetDpopNonce() (string, bool, error) {
	return n.nonce, true, nil
}

// SetDpopNonce implements auth.DpopNonceProvider.
func (n *nonceProvider) SetDpopNonce(nonce string) error {
	n.nonce = nonce
	return nil
}
