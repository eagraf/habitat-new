package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/gorilla/sessions"
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

type ErrorSessionValueNotFound struct {
	Key string
}

func (e *ErrorSessionValueNotFound) Error() string {
	return fmt.Sprintf("session value not found: %s", e.Key)
}

type dpopSession struct {
	session *sessions.Session

	// These are needed for saving the session
	req        *http.Request
	respWriter http.ResponseWriter
}

func createFreshDpopSession(
	r *http.Request,
	w http.ResponseWriter,
	sessionStore sessions.Store,
	id *identity.Identity,
	pdsURL string,
) (*dpopSession, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}

	identityBytes, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}

	session, err := sessionStore.New(r, "dpop-session")
	if err != nil {
		return nil, err
	}

	session.Values[cKeySessionKey] = keyBytes
	session.Values[cIdentitySessionKey] = identityBytes
	session.Values[cPDSURLSessionKey] = pdsURL

	err = session.Save(r, w)
	if err != nil {
		return nil, err
	}

	return &dpopSession{session: session}, nil
}

func getExistingDpopSession(r *http.Request, w http.ResponseWriter, sessionStore sessions.Store) (*dpopSession, error) {
	session, err := sessionStore.Get(r, "dpop-session")
	if err != nil {
		return nil, err
	}

	// Check that the session has a valid key, so we can fail fast if the session is somehow invalid
	_, ok := session.Values[cKeySessionKey]
	if !ok {
		return nil, errors.New("invalid/missing key in session")
	}

	return &dpopSession{session: session, req: r, respWriter: w}, nil
}

func (s *dpopSession) generateDpop(key *ecdsa.PrivateKey) error {
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	s.session.Values[cKeySessionKey] = keyBytes
	return nil
}

func getDpopKey(session *sessions.Session) (*ecdsa.PrivateKey, error) {
	keyBytes, ok := session.Values[cKeySessionKey]
	if !ok {
		return nil, errors.New("invalid/missing key in session")
	}
	bytes, ok := keyBytes.([]byte)
	if !ok {
		return nil, errors.New("key in session is not a []byte")
	}
	return x509.ParseECPrivateKey(bytes)
}

func (s *dpopSession) setTokenResponseFields(tokenResp *TokenResponse) error {
	// Set the access token hash, which is used in claims for future DPoP requests
	h := sha256.New()
	h.Write([]byte(tokenResp.AccessToken))
	hash := h.Sum(nil)
	encodedHash := base64.RawURLEncoding.EncodeToString(hash)
	s.session.Values[cAccessTokenHashSessionKey] = encodedHash

	s.session.Values[cAccessTokenSessionKey] = tokenResp.AccessToken
	s.session.Values[cRefreshTokenSessionKey] = tokenResp.RefreshToken

	err := s.session.Save(s.req, s.respWriter)
	if err != nil {
		return err
	}
	return nil
}

func (s *dpopSession) getAccessTokenHash() (string, error) {
	v, ok := s.session.Values[cAccessTokenHashSessionKey]
	if !ok {
		return "", &ErrorSessionValueNotFound{Key: cAccessTokenHashSessionKey}
	}
	hash, ok := v.(string)
	if !ok {
		return "", errors.New("access token hash in session is not a string")
	}
	return hash, nil
}

func (s *dpopSession) getAccessToken() (string, error) {
	v, ok := s.session.Values[cAccessTokenSessionKey]
	if !ok {
		return "", &ErrorSessionValueNotFound{Key: cAccessTokenSessionKey}
	}
	token, ok := v.(string)
	if !ok {
		return "", errors.New("access token in session is not a string")
	}
	return token, nil
}

func (s *dpopSession) getRefreshToken() (string, error) {
	v, ok := s.session.Values[cRefreshTokenSessionKey]
	if !ok {
		return "", &ErrorSessionValueNotFound{Key: cRefreshTokenSessionKey}
	}
	token, ok := v.(string)
	if !ok {
		return "", errors.New("refresh token in session is not a string")
	}
	return token, nil
}

func (s *dpopSession) setIssuer(issuer string) error {
	s.session.Values[cIssuerSessionKey] = issuer
	err := s.session.Save(s.req, s.respWriter)
	if err != nil {
		return err
	}
	return nil
}

func (s *dpopSession) getIssuer() (string, error) {
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

func (s *dpopSession) getPDSURL() (string, error) {
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

func (s *dpopSession) getIdentity() (*identity.Identity, error) {
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

func (s *dpopSession) setNonce(nonce string) {
	s.session.Values[cNonceSessionKey] = nonce
}

func (s *dpopSession) getNonce() (string, error) {
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
