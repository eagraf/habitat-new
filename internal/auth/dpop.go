package auth

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
)

const (
	cKeySessionKey             = "key"
	cNonceSessionKey           = "nonce"
	cAccessTokenHashSessionKey = "ath"
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
	session *sessions.Session
}

func NewDpopHttpClient(session *sessions.Session) *DpopHttpClient {
	return &DpopHttpClient{session}
}

func (s *DpopHttpClient) Do(req *http.Request) (*http.Response, error) {
	s.sign(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get("DPoP-Nonce") != "" {
		s.session.Values[cNonceSessionKey] = resp.Header.Get("DPoP-Nonce")
	}
	if !isUseDPopNonceError(resp) {
		return resp, nil
	}
	log.Info().Msgf("retrying with new nonce: %s", resp.Header.Get("DPoP-Nonce"))
	// retry with new nonce
	s.sign(req)
	return http.DefaultClient.Do(req)
}

func (s *DpopHttpClient) SetKey(key *ecdsa.PrivateKey) {
	s.session.Values[cKeySessionKey] = key
}

func (s *DpopHttpClient) SetAccessTokenHash(ath string) {
	s.session.Values[cAccessTokenHashSessionKey] = ath
}

func (s *DpopHttpClient) sign(req *http.Request) error {
	key, ok := s.session.Values[cKeySessionKey].(*ecdsa.PrivateKey)
	if !ok {
		return errors.New("invalid/missing key in session")
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
	accessTokenHash, _ := s.session.Values[cAccessTokenHashSessionKey].(string)
	token, err := jwt.Signed(signer).Claims(&dpopClaims{
		Claims: jwt.Claims{
			ID:       generateNonce(),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		Method:          req.Method,
		URL:             req.URL.String(),
		Nonce:           nonce,
		AccessTokenHash: accessTokenHash,
	}).CompactSerialize()
	if err != nil {
		return err
	}
	log.Info().
		Str("nonce", nonce).
		Str("url", req.URL.String()).
		Str("method", req.Method).
		Str("token", token).
		Str("access_token_hash", accessTokenHash).
		Msg("signing request")
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
		var buf [1024]byte
		_, err := resp.Body.Read(buf[:])
		if err == nil {
			var respBody map[string]any
			err := json.NewDecoder(bytes.NewReader(buf[:])).Decode(&respBody)
			if err == nil {
				if respBody["error"] == "use_dpop_nonce" {
					return true
				}
			}
			log.Error().Err(err).Msg("error decoding response body")
		}
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf[:]), resp.Body))
	}
	return false
}
