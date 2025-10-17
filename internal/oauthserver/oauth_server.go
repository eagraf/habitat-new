package oauthserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net/http"

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
	handle  string
	request fosite.AuthorizeRequester
	state   *auth.AuthorizeState
	dpopKey *ecdsa.PrivateKey
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
	secret := []byte("my super secret signing password")
	config := &fosite.Config{
		GlobalSecret: secret,
	}
	provider := compose.Compose(
		config,
		storage,
		compose.NewOAuth2HMACStrategy(config),
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2PKCEFactory,
	)

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
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}
	dpopClient := auth.NewDpopHttpClient(key, &nonceProvider{})
	redirect, state, err := o.oauthClient.Authorize(dpopClient, id)
	if err != nil {
		return fmt.Errorf("failed to authorize: %w", err)
	}

	session, _ := o.sessionStore.Get(r, sessionName)
	session.AddFlash(&authRequestFlash{
		dpopKey: key,
		handle:  handle,
		request: requester,
		state:   state,
	})
	session.Save(r, w)

	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

func (o *OAuthServer) HandleCallback(
	w http.ResponseWriter,
	r *http.Request,
) error {
	ctx := r.Context()
	session, err := o.sessionStore.Get(r, sessionName)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	arf := session.Flashes()[0].(*authRequestFlash)

	dpopClient := auth.NewDpopHttpClient(arf.dpopKey, &nonceProvider{})
	tokenInfo, err := o.oauthClient.ExchangeCode(
		dpopClient,
		r.URL.Query().Get("code"),
		r.URL.Query().Get("iss"),
		arf.state,
	)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}
	resp, err := o.provider.NewAuthorizeResponse(
		ctx,
		arf.request,
		newSession(arf.handle, arf.dpopKey, tokenInfo),
	)
	o.provider.WriteAuthorizeResponse(r.Context(), w, arf.request, resp)
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
