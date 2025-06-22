package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type ClientMetadata struct {
	ClientName              string              `json:"client_name"`
	ClientId                string              `json:"client_id"`
	ClientUri               string              `json:"client_uri"`
	ApplicationType         string              `json:"application_type"`
	GrantTypes              []string            `json:"grant_types"`
	Scope                   string              `json:"scope"`
	ResponseTypes           []string            `json:"response_types"`
	RedirectUris            []string            `json:"redirect_uris"`
	TokenEndpointAuthMethod string              `json:"token_endpoint_auth_method"`
	TokenEndpointAuthSigner string              `json:"token_endpoint_auth_signing_alg"`
	DpopBoundAccessTokens   bool                `json:"dpop_bound_access_tokens"`
	Jwks                    *jose.JSONWebKeySet `json:"jwks"`
}

type OAuthClient interface {
	ClientMetadata() *ClientMetadata
	Authorize(dpopClient *DpopHttpClient, i *identity.Identity, handle *string) (string, *AuthorizeState, error)
	ExchangeCode(
		dpopClient *DpopHttpClient,
		code string,
		issuer string,
		state *AuthorizeState,
	) (*TokenResponse, error)
}

type oauthClientImpl struct {
	clientId    string
	redirectUri string
	secretJwk   jose.JSONWebKey
}

func NewOAuthClient(clientId string, redirectUri string, secretJwk []byte) (OAuthClient, error) {
	var secret jose.JSONWebKey
	err := json.Unmarshal(secretJwk, &secret)
	if err != nil {
		return nil, err
	}
	return &oauthClientImpl{
		clientId:    clientId,
		redirectUri: redirectUri,
		secretJwk:   secret,
	}, nil
}

// ClientMetadata implements OAuthClient.
func (o *oauthClientImpl) ClientMetadata() *ClientMetadata {
	publicJwk := o.secretJwk.Public()
	return &ClientMetadata{
		ClientName:              "Habitat",
		ClientUri:               "https://beacon-dev.tail07d32.ts.net",
		ClientId:                o.clientId,
		ApplicationType:         "web",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		Scope:                   "atproto transition:generic",
		ResponseTypes:           []string{"code"},
		RedirectUris:            []string{o.redirectUri},
		TokenEndpointAuthMethod: "private_key_jwt",
		TokenEndpointAuthSigner: "ES256",
		DpopBoundAccessTokens:   true,
		Jwks:                    &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{publicJwk}},
	}
}

type AuthorizeState struct {
	Verifier      string `json:"v"`
	State         string `json:"s"`
	TokenEndpoint string `json:"te"`
}

// Authorize implements OAuthClient.
func (o *oauthClientImpl) Authorize(
	dpopClient *DpopHttpClient,
	i *identity.Identity,
	loginHint *string,
) (string, *AuthorizeState, error) {
	pr, err := fetchOAuthProtectedResource(i)
	if err != nil {
		return "", nil, err
	}

	serverMetadata, err := fetchOauthAuthorizationServer(i, pr)
	if err != nil {
		return "", nil, err
	}

	verifier := oauth2.GenerateVerifier()

	stateBytes := make([]byte, 12)
	_, err = rand.Read(stateBytes)
	if err != nil {
		return "", nil, err
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	requestUri, err := o.makePushedAuthorizationRequest(
		dpopClient,
		loginHint,
		serverMetadata,
		state,
		verifier,
	)
	if err != nil {
		return "", nil, err
	}

	redirectUrl, err := url.Parse(serverMetadata.AuthEndpoint)
	redirectUrl.RawQuery = url.Values{
		"client_id":   {o.clientId},
		"request_uri": {requestUri},
	}.Encode()

	return redirectUrl.String(), &AuthorizeState{
		Verifier:      verifier,
		State:         state,
		TokenEndpoint: serverMetadata.TokenEndpoint,
	}, nil
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeCode implements OAuthClient.
func (o *oauthClientImpl) ExchangeCode(
	dpopClient *DpopHttpClient,
	code string,
	issuer string,
	state *AuthorizeState,
) (*TokenResponse, error) {
	clientAssertion, err := o.getClientAssertion(issuer)
	if err != nil {
		return nil, err
	}

	log.Info().Msgf("token endpoint: %s", state.TokenEndpoint)
	req, err := http.NewRequest(
		http.MethodPost,
		state.TokenEndpoint,
		strings.NewReader(url.Values{
			"client_id":     {o.clientId},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {o.redirectUri},
			"code":          {code},
			"code_verifier": {state.Verifier},

			"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
			"client_assertion":      {clientAssertion},
		}.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := dpopClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errMsg json.RawMessage
		json.NewDecoder(resp.Body).Decode(&errMsg)
		return nil, fmt.Errorf("failed to exchange code: %s", resp.Status, string(errMsg))
	}

	rawTokenResp, err := io.ReadAll(resp.Body)

	var tokenResp TokenResponse
	err = json.NewDecoder(bytes.NewReader(rawTokenResp)).Decode(&tokenResp)
	if err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

type oauthProtectedResource struct {
	AuthorizationServers []string `json:"authorization_servers"`
}

func fetchOAuthProtectedResource(i *identity.Identity) (*oauthProtectedResource, error) {
	url, err := url.Parse(i.PDSEndpoint())
	if err != nil {
		return nil, err
	}
	if url.Host == "localhost:3000" {
		url.Host = "host.docker.internal:3000"
	}
	resp, err := http.DefaultClient.Get(
		url.JoinPath("/.well-known/oauth-protected-resource").String(),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to fetch authorization server",
			errors.New(resp.Status),
		)
	}

	var pr oauthProtectedResource
	json.NewDecoder(resp.Body).Decode(&pr)

	return &pr, nil
}

type oauthAuthorizationServer struct {
	Issuer        string `json:"issuer"`
	TokenEndpoint string `json:"token_endpoint"`
	PAREndpoint   string `json:"pushed_authorization_request_endpoint"`
	AuthEndpoint  string `json:"authorization_endpoint"`
}

func fetchOauthAuthorizationServer(
	i *identity.Identity,
	pr *oauthProtectedResource,
) (*oauthAuthorizationServer, error) {
	if len(pr.AuthorizationServers) == 0 {
		return nil, errors.New("no authorization server found")
	}
	url, err := url.Parse(pr.AuthorizationServers[0])
	if err != nil {
		return nil, err
	}
	if url.Host == "localhost:3000" {
		url.Host = "host.docker.internal:5001"
	}
	url.Path = "/.well-known/oauth-authorization-server"
	resp, err := http.DefaultClient.Get(
		url.String(),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to fetch authorization server",
			errors.New(resp.Status),
		)
	}

	var as oauthAuthorizationServer
	err = json.NewDecoder(resp.Body).Decode(&as)
	if err != nil {
		return nil, err
	}

	return &as, nil
}

type parResponse struct {
	RequestUri string `json:"request_uri"`
}

func (o *oauthClientImpl) getClientAssertion(audience string) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: o.secretJwk},
		&jose.SignerOptions{
			ExtraHeaders: map[jose.HeaderKey]interface{}{
				"kid": o.secretJwk.KeyID,
			},
		},
	)
	if err != nil {
		return "", err
	}

	return jwt.Signed(signer).Claims(&jwt.Claims{
		Issuer:   o.clientId,
		Subject:  o.clientId,
		Audience: jwt.Audience{audience},
		Expiry:   jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ID:       generateNonce(),
	}).CompactSerialize()
}

func (o *oauthClientImpl) makePushedAuthorizationRequest(
	dpopClient *DpopHttpClient,
	loginHint *string,
	as *oauthAuthorizationServer,
	state string,
	verifier string,
) (string, error) {
	log.Info().Msgf("issuer: %s", as.Issuer)
	clientAssertion, err := o.getClientAssertion(as.Issuer)
	if err != nil {
		return "", err
	}

	parUrl, err := url.Parse(as.PAREndpoint)
	if err != nil {
		return "", err
	}
	if parUrl.Host == "localhost:3000" {
		parUrl.Host = "host.docker.internal:5001"
	}

	params := url.Values{
		"client_id":             {o.clientId},
		"redirect_uri":          {o.redirectUri},
		"code_challenge":        {oauth2.S256ChallengeFromVerifier(verifier)},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"respose_mode":          {"query"},
		"response_type":         {"code"},
		"scope":                 {"atproto transition:generic"},
		//"login_hint":            {*loginHint},

		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {clientAssertion},
	}

	if loginHint != nil {
		params.Add("login_hint", *loginHint)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		parUrl.String(),
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.Warn().
		Str("client assertion", clientAssertion).
		Str("issuer", as.Issuer).
		Str("par url", parUrl.String()).
		Str("login hint", *loginHint).
		Str("state", state).
		Str("verifier", verifier).
		Str("redirect uri", o.redirectUri).
		Msg("making par request")

	resp, err := dpopClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		errMsg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"failed to make pushed authorization request",
			resp.Status,
			string(errMsg),
		)
	}

	var respBody parResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", err
	}

	return respBody.RequestUri, nil
}
