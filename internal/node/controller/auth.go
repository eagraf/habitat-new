package controller

import (
	"context"
	"net/http"

	"github.com/eagraf/go-atproto-oauth/oauth"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/rs/zerolog/log"
)

type AuthenticationMiddleware struct {
	nodeController NodeController
	useTLS         bool
	authPersister  oauth.Persister
}

func NewAuthenticationMiddleware(ctrl NodeController, useTLS bool, authPersister oauth.Persister) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{
		nodeController: ctrl,
		useTLS:         useTLS,
		authPersister:  authPersister,
	}
}

func (amw *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		var username string
		var userID string

		// Get session ID from cookie
		sessionId, err := r.Cookie("state")
		if err != nil {
			http.Error(w, "No session cookie found", http.StatusUnauthorized)
			return
		}

		_, err = r.Cookie("handle")
		if err != nil {
			http.Error(w, "No handle cookie found", http.StatusUnauthorized)
			return
		}

		_, err = amw.authPersister.GetSession(sessionId.Value)
		if err != nil {
			http.Error(w, "No session found", http.StatusUnauthorized)
			return
		}

		// For now, hardcode root user credentials when session cookie is present
		// TODO: Implement proper session management and user lookup
		username = "root"
		userID = "0"

		log.Info().Msgf("Authenticated user: %s", username)

		ctx := r.Context()
		newCtx := context.WithValue(ctx, constants.ContextKeyUserID, userID)

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}
