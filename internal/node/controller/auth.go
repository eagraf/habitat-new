package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/rs/zerolog/log"
)

type AuthenticationMiddleware struct {
	nodeController NodeController
	useTLS         bool
	rootUserCert   *x509.Certificate
}

func NewAuthenticationMiddleware(ctrl NodeController, useTLS bool, rootUserCert *x509.Certificate) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{
		nodeController: ctrl,
		useTLS:         useTLS,
		rootUserCert:   rootUserCert,
	}
}

func (amw *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		var username string
		var userID string

		// Only authenticate if TLS is enabled. This is temporary.
		if amw.useTLS {
			if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.VerifiedChains[0]) == 0 {
				http.Error(w, "No client certificate found", http.StatusUnauthorized)
				return
			}

			// TODO we probably need to loop through all verified chains in the future
			reqCert := r.TLS.VerifiedChains[0][0]
			username := reqCert.Subject.CommonName

			var storedCert *x509.Certificate
			if username == constants.RootUsername {
				storedCert = amw.rootUserCert
				username = constants.RootUsername
				userID = constants.RootUserID
			} else {
				userCert, user, err := getUserCertAndInfo(amw.nodeController, username)
				if err != nil {
					http.Error(w, fmt.Sprintf("Error getting user certificate: %s", err), http.StatusInternalServerError)
					return
				}
				userID = user.ID
				storedCert = userCert
			}

			if !reqCert.Equal(storedCert) {
				http.Error(w, fmt.Sprintf("Certificate in request did not match certificate for username %s", username), http.StatusInternalServerError)
				return
			}
		} else {
			// TODO @eagraf - figure out a better way to make authenticating in dev mode more expedient
			username = "root"
			userID = "0"
		}

		log.Info().Msgf("Authenticated user: %s", username)

		ctx := r.Context()
		newCtx := context.WithValue(ctx, constants.ContextKeyUserID, userID)

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}

func getUserCertAndInfo(controller NodeController, username string) (*x509.Certificate, *node.User, error) {
	// Look up the user in the node's user list
	user, err := controller.GetUserByUsername(username)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting user: %s", err)
	}

	block, _ := pem.Decode([]byte(user.Certificate))
	if block == nil {
		return nil, nil, errors.New("got nil block after decoding PEM")
	}

	if block.Type != "CERTIFICATE" {
		return nil, nil, errors.New("expected CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, user, nil
}
