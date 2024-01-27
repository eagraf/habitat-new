package api

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/rs/zerolog/log"
)

var RootUsername = "root"

type authenticationMiddleware struct {
	nodeController *controller.NodeController
	nodeConfig     *config.NodeConfig
}

func (amw *authenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.VerifiedChains[0]) == 0 {
			http.Error(w, "No client certificate found", http.StatusUnauthorized)
			return
		}

		// TODO we probably need to loop through all verified chains in the future
		reqCert := r.TLS.VerifiedChains[0][0]
		username := reqCert.Subject.CommonName

		var storedCert *x509.Certificate
		if username == RootUsername {
			storedCert = amw.nodeConfig.RootUserCert()
			username = RootUsername
		} else {
			userCert, err := getUserCert(amw.nodeController, username)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error getting user certificate: %s", err), http.StatusInternalServerError)
				return
			}
			storedCert = userCert
		}

		if !reqCert.Equal(storedCert) {
			http.Error(w, fmt.Sprintf("Certificate in request did not match certificate for username %s", username), http.StatusInternalServerError)
			return
		}

		log.Info().Msgf("Authenticated user: %s", username)

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r)
	})
}

func getUserCert(controller *controller.NodeController, username string) (*x509.Certificate, error) {
	// Look up the user in the node's user list
	user, err := controller.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("error getting user: %s", err)
	}

	block, _ := pem.Decode([]byte(user.Certificate))
	if block == nil {
		return nil, errors.New("Got nil block after decoding PEM")
	}

	if block.Type != "CERTIFICATE" {
		return nil, errors.New("Expected CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
