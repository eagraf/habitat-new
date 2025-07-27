package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type Session interface {
	GetDpopKey() (*ecdsa.PrivateKey, error)
	SetDpopKey(*ecdsa.PrivateKey)

	GetNonce() (string, error)
	SetNonce(string)

	GetTokenInfo() (*TokenResponse, error)
	SetTokenInfo(*TokenResponse) error

	GetIdentity() (*identity.Identity, error)
	SetIdentity(*identity.Identity) error

	GetPDSURL() (string, error)
	SetPDSURL(string)

	GetIssuer() (string, error)
	SetIssuer(string)
}

const (
	cKeySessionKey       = "key"
	cNonceSessionKey     = "nonce"
	cTokenInfoSessionKey = "token_info"
	cIssuerSessionKey    = "issuer"
	cIdentitySessionKey  = "identity"
	cPDSURLSessionKey    = "pds_url"
)

type ErrorSessionValueNotFound struct {
	Key string
}

func (e *ErrorSessionValueNotFound) Error() string {
	return fmt.Sprintf("session value not found: %s", e.Key)
}

type habitatGorillaSession struct {
	session *sessions.Session

	// These are needed for saving the session
	req        *http.Request
	respWriter http.ResponseWriter
}

func newHabitatGorillaSession(
	r *http.Request,
	w http.ResponseWriter,
	sessionStore sessions.Store,
	id *identity.Identity,
	pdsURL string,
) (*habitatGorillaSession, error) {
	session, err := sessionStore.New(r, "dpop-session")
	if err != nil {
		return nil, err
	}

	// TODO inject the key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	dpopSession := &habitatGorillaSession{session: session, req: r, respWriter: w}

	// Use session class methods instead of manually modifying session values
	err = dpopSession.SetDpopKey(key)
	if err != nil {
		return nil, err
	}

	err = dpopSession.SetIdentity(id)
	if err != nil {
		return nil, err
	}

	err = dpopSession.SetPDSURL(pdsURL)
	if err != nil {
		return nil, err
	}

	return dpopSession, nil
}

func getExistingHabitatGorillaSession(r *http.Request, w http.ResponseWriter, sessionStore sessions.Store) (*habitatGorillaSession, error) {
	session, err := sessionStore.Get(r, "dpop-session")
	if err != nil {
		return nil, err
	}

	// Check that the session has a valid key, so we can fail fast if the session is somehow invalid
	_, ok := session.Values[cKeySessionKey]
	if !ok {
		return nil, errors.New("invalid/missing key in session")
	}

	return &habitatGorillaSession{session: session, req: r, respWriter: w}, nil
}

// Save writes the session out to the response writer. It is designed to be called with defer.
func (s *habitatGorillaSession) Save(r *http.Request, w http.ResponseWriter) {
	err := s.session.Save(r, w)
	if err != nil {
		log.Error().Err(err).Msg("error saving session")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			log.Error().Err(err).Msg("error writing error to response")
		}
	}
}

func (s *habitatGorillaSession) SetDpopKey(key *ecdsa.PrivateKey) error {
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	s.session.Values[cKeySessionKey] = keyBytes
	return nil
}

func (s *habitatGorillaSession) GetDpopKey() (*ecdsa.PrivateKey, error) {
	keyBytes, ok := s.session.Values[cKeySessionKey]
	if !ok {
		return nil, errors.New("invalid/missing key in session")
	}
	bytes, ok := keyBytes.([]byte)
	if !ok {
		return nil, errors.New("key in session is not a []byte")
	}
	return x509.ParseECPrivateKey(bytes)
}

func (s *habitatGorillaSession) SetTokenInfo(tokenResp *TokenResponse) error {
	marshalled, err := json.Marshal(tokenResp)
	if err != nil {
		return err
	}

	s.session.Values[cTokenInfoSessionKey] = marshalled

	return nil
}

func (s *habitatGorillaSession) GetTokenInfo() (*TokenResponse, error) {
	marshalled, ok := s.session.Values[cTokenInfoSessionKey]
	if !ok {
		return nil, &ErrorSessionValueNotFound{Key: cTokenInfoSessionKey}
	}
	bytes, ok := marshalled.([]byte)
	if !ok {
		return nil, errors.New("token info in session is not a []byte")
	}
	var tokenResp TokenResponse
	err := json.Unmarshal(bytes, &tokenResp)
	if err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

func (s *habitatGorillaSession) SetIssuer(issuer string) error {
	s.session.Values[cIssuerSessionKey] = issuer
	return nil
}

func (s *habitatGorillaSession) GetIssuer() (string, error) {
	v, ok := s.session.Values[cIssuerSessionKey]
	if !ok {
		return "", &ErrorSessionValueNotFound{Key: cIssuerSessionKey}
	}
	issuer, ok := v.(string)
	if !ok {
		return "", errors.New("issuer in session is not a string")
	}
	return issuer, nil
}

func (s *habitatGorillaSession) SetPDSURL(pdsURL string) error {
	s.session.Values[cPDSURLSessionKey] = pdsURL
	return nil
}

func (s *habitatGorillaSession) GetPDSURL() (string, error) {
	v, ok := s.session.Values[cPDSURLSessionKey]
	if !ok {
		return "", &ErrorSessionValueNotFound{Key: cPDSURLSessionKey}
	}
	url, ok := v.(string)
	if !ok {
		return "", errors.New("PDS URL in session is not a string")
	}
	return url, nil
}

func (s *habitatGorillaSession) GetIdentity() (*identity.Identity, error) {
	bytes, ok := s.session.Values[cIdentitySessionKey].([]byte)
	if !ok {
		return nil, &ErrorSessionValueNotFound{Key: cIdentitySessionKey}
	}
	var i identity.Identity
	err := json.Unmarshal(bytes, &i)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *habitatGorillaSession) SetIdentity(id *identity.Identity) error {
	identityBytes, err := json.Marshal(id)
	if err != nil {
		return err
	}
	s.session.Values[cIdentitySessionKey] = identityBytes
	return nil
}

func (s *habitatGorillaSession) SetNonce(nonce string) error {
	s.session.Values[cNonceSessionKey] = nonce
	return nil
}

func (s *habitatGorillaSession) GetNonce() (string, error) {
	v, ok := s.session.Values[cNonceSessionKey]
	if !ok {
		return "", &ErrorSessionValueNotFound{Key: cNonceSessionKey}
	}
	nonce, ok := v.(string)
	if !ok {
		return "", errors.New("nonce in session is not a string")
	}
	return nonce, nil
}

// SessionNonceProvider adapts dpopSession to implement NonceProvider
type SessionNonceProvider struct {
	session *habitatGorillaSession
}

// NewSessionNonceProvider creates a new SessionNonceProvider from a dpopSession
func NewSessionNonceProvider(session *habitatGorillaSession) *SessionNonceProvider {
	return &SessionNonceProvider{session: session}
}

// GetNonce implements NonceProvider
func (p *SessionNonceProvider) GetNonce() (string, bool, error) {
	nonce, err := p.session.GetNonce()
	if err != nil {
		return "", false, nil
	}
	return nonce, true, nil
}

// SetNonce implements NonceProvider
func (p *SessionNonceProvider) SetNonce(nonce string) error {
	return p.session.SetNonce(nonce)
}
