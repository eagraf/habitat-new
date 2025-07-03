package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/atproto/identity"
	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
)

const (
	cKeySessionKey             = "key"
	cNonceSessionKey           = "nonce"
	cAccessTokenHashSessionKey = "ath"

	cAccessTokenSessionKey  = "access_token"
	cRefreshTokenSessionKey = "refresh_token"
	cIssuerSessionKey       = "issuer"
	cIdentitySessionKey     = "identity"
	cPDSURLSessionKey       = "pds_url"
)

type dpopClaims struct {
	jwt.Claims

	// the `htm` (HTTP Method) claim. See https://datatracker.ietf.org/doc/html/draft-ietf-oauth-dpop#section-4.2
	Method string `json:"htm"`

	// the `htu` (HTTP URL) claim. See https://datatracker.ietf.org/doc/html/draft-ietf-oauth-dpop#section-4.2
	URL string `json:"htu"`

	// the `ath` (Authorization Token Hash) claim. See https://datatracker.ietf.org/doc/html/draft-ietf-oauth-dpop#section-4.2
	AccessTokenHash string `json:"ath,omitempty"`

	// the `nonce` claim. See https://datatracker.ietf.org/doc/html/draft-ietf-oauth-dpop#section-4.2
	Nonce string `json:"nonce,omitempty"`
}

type DpopHttpClient struct {
	htu        string
	session    *sessions.Session
	jwkBuilder DpopJWKBuilder
}

func NewDpopHttpClient(session *sessions.Session, jwkBuilder DpopJWKBuilder) *DpopHttpClient {
	return &DpopHttpClient{session: session, jwkBuilder: jwkBuilder}
}

func (s *DpopHttpClient) Do(req *http.Request) (*http.Response, error) {
	err := s.Sign(req)
	if err != nil {
		return nil, err
	}
	// Read out the body since we'll need it twice
	bodyBytes := []byte{}
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check if new nonce is needed, and set it if so
	if !isUseDPopNonceError(resp) {
		return resp, nil
	}
	if resp.Header.Get("DPoP-Nonce") != "" {
		s.session.Values[cNonceSessionKey] = resp.Header.Get("DPoP-Nonce")
	}

	// retry with new nonce
	req2 := req.Clone(req.Context())
	req2.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	err = s.Sign(req2)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req2)
}

func (s *DpopHttpClient) SetKey(key *ecdsa.PrivateKey) error {
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	s.session.Values[cKeySessionKey] = keyBytes
	return nil
}

func (s *DpopHttpClient) SetAccessTokenHash(accessToken string) {
	h := sha256.New()
	h.Write([]byte(accessToken))
	hash := h.Sum(nil)
	encodedHash := base64.RawURLEncoding.EncodeToString(hash)
	log.Info().Msgf("setting accessTokenHash: %s", encodedHash)
	s.session.Values[cAccessTokenHashSessionKey] = encodedHash
}

func (s *DpopHttpClient) SetAccessToken(accessToken string) {
	s.session.Values[cAccessTokenSessionKey] = accessToken
}

func (s *DpopHttpClient) GetAccessToken() string {
	return s.session.Values[cAccessTokenSessionKey].(string)
}

func (s *DpopHttpClient) SetRefreshToken(refreshToken string) {
	s.session.Values[cRefreshTokenSessionKey] = refreshToken
}

func (s *DpopHttpClient) GetRefreshToken() string {
	return s.session.Values[cRefreshTokenSessionKey].(string)
}

func (s *DpopHttpClient) SetIssuer(issuer string) {
	s.session.Values[cIssuerSessionKey] = issuer
}

func (s *DpopHttpClient) GetIssuer() string {
	return s.session.Values[cIssuerSessionKey].(string)
}

func (s *DpopHttpClient) SetPDSURL(pdsURL string) {
	s.session.Values[cPDSURLSessionKey] = pdsURL
}

func (s *DpopHttpClient) GetPDSURL() string {
	if v, ok := s.session.Values[cPDSURLSessionKey]; ok {
		return v.(string)
	}
	return ""
}

func (s *DpopHttpClient) SetIdentity(i *identity.Identity) error {
	bytes, err := json.Marshal(i)
	if err != nil {
		return err
	}
	s.session.Values[cIdentitySessionKey] = bytes
	return nil
}

func (s *DpopHttpClient) GetIdentity() (*identity.Identity, error) {
	bytes, ok := s.session.Values[cIdentitySessionKey].([]byte)
	if !ok {
		return nil, errors.New("invalid/missing identity in session")
	}
	var i identity.Identity
	err := json.Unmarshal(bytes, &i)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *DpopHttpClient) Sign(req *http.Request) error {
	keyBytes, ok := s.session.Values[cKeySessionKey]
	if !ok {
		return errors.New("invalid/missing key in session")
	}
	key, err := x509.ParseECPrivateKey(keyBytes.([]byte))
	if err != nil {
		return err
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		&jose.SignerOptions{
			ExtraHeaders: map[jose.HeaderKey]any{
				jose.HeaderType: "dpop+jwt",
				"jwk": &jose.JSONWebKey{
					Key:       key.Public(),
					Use:       "sig",
					Algorithm: string(jose.ES256),
				},
			},
		},
	)
	if err != nil {
		return err
	}

	nonce, _ := s.session.Values[cNonceSessionKey].(string)
	token, err := s.jwkBuilder(req, signer, s.session, nonce)
	if err != nil {
		return err
	}
	req.Header.Set("DPoP", token)
	return nil
}

func isUseDPopNonceError(resp *http.Response) bool {
	// Resource server
	if resp.StatusCode == 401 {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if strings.HasPrefix(wwwAuth, "DPoP") {
			return strings.Contains(wwwAuth, "error=\"use_dpop_nonce\"")
		}
	}

	// Authorization server
	if resp.StatusCode == 400 {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			var respBody map[string]any
			err := json.NewDecoder(bytes.NewReader(body)).Decode(&respBody)
			if err == nil {
				if respBody["error"] == "use_dpop_nonce" {
					return true
				}
			}
			log.Error().Err(err).Msg("error decoding response body")
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	return false
}
