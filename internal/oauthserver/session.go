package oauthserver

import (
	"crypto/ecdsa"
	"time"

	"github.com/eagraf/habitat-new/internal/auth"
	"github.com/ory/fosite"
)

type session struct {
	expiresAt    time.Time
	dpopKey      *ecdsa.PrivateKey
	accessToken  string
	refreshToken string
	subject      string
}

var _ fosite.Session = (*session)(nil)

func newSession(subject string, dpopKey *ecdsa.PrivateKey, tokenInfo *auth.TokenResponse) *session {
	return &session{
		subject:   subject,
		dpopKey:   dpopKey,
		expiresAt: time.Now().UTC().Add(time.Duration(tokenInfo.ExpiresIn)),
	}
}

// Clone implements fosite.Session.
func (s *session) Clone() fosite.Session {
	return s
}

// GetExpiresAt implements fosite.Session.
func (s *session) GetExpiresAt(key fosite.TokenType) time.Time {
	return s.expiresAt
}

// GetSubject implements fosite.Session.
func (s *session) GetSubject() string {
	return s.subject
}

// GetUsername implements fosite.Session.
func (s *session) GetUsername() string {
	return s.subject
}

// SetExpiresAt implements fosite.Session.
func (s *session) SetExpiresAt(key fosite.TokenType, exp time.Time) {
	panic("unimplemented")
}
