package auth

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/rs/zerolog/log"
)

// NonceProvider provides access to nonce management
type NonceProvider interface {
	GetNonce() (string, error)
	SetNonce(nonce string)
}

// DpopJWKBuilder is a function that builds a DPoP JWT for a given request type.
// DpopHttpClient will invoke it before making a request to get a signed JWK with proper info.
type DpopJWKBuilder func(req *http.Request, signer jose.Signer, nonce string) (string, error)

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
	htu           string
	key           *ecdsa.PrivateKey
	nonceProvider NonceProvider
	jwkBuilder    DpopJWKBuilder
}

func NewDpopHttpClient(key *ecdsa.PrivateKey, nonceProvider NonceProvider, jwkBuilder DpopJWKBuilder) *DpopHttpClient {
	return &DpopHttpClient{key: key, nonceProvider: nonceProvider, jwkBuilder: jwkBuilder}
}

func (s *DpopHttpClient) Do(req *http.Request) (*http.Response, error) {
	err := s.sign(req)
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
		s.nonceProvider.SetNonce(resp.Header.Get("DPoP-Nonce"))
	}

	// retry with new nonce
	req2 := req.Clone(req.Context())
	req2.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	err = s.sign(req2)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req2)
}

func (s *DpopHttpClient) sign(req *http.Request) error {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: s.key},
		&jose.SignerOptions{
			ExtraHeaders: map[jose.HeaderKey]any{
				jose.HeaderType: "dpop+jwt",
				"jwk": &jose.JSONWebKey{
					Key:       s.key.Public(),
					Use:       "sig",
					Algorithm: string(jose.ES256),
				},
			},
		},
	)
	if err != nil {
		return err
	}

	nonce, err := s.nonceProvider.GetNonce()
	var notFoundErr *ErrorSessionValueNotFound
	if err != nil && !errors.As(err, &notFoundErr) {
		return err
	}
	token, err := s.jwkBuilder(req, signer, nonce)
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
