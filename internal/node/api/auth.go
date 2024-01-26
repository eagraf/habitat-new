package api

import (
	"crypto/x509"
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/rs/zerolog/log"
)

type authenticationMiddleware struct {
	nodeController *controller.NodeController
}

func (amw *authenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msg("Authenticating user")
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.VerifiedChains[0]) == 0 {
			http.Error(w, "No client certificate found", http.StatusUnauthorized)
			return
		}

		// TODO we probably need to loop through all verified chains in the future
		reqCert := r.TLS.VerifiedChains[0]
		commonName := r.TLS.VerifiedChains[0][0].Subject.CommonName

		// Look up the user in the node's user list
		user, err := amw.nodeController.GetUserByUsername(commonName)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		userCert, err := x509.ParseCertificate([]byte(user.Certificate))
		if err != nil {
			http.Error(w, "Error parsing user certificate", http.StatusUnauthorized)
			return
		}

		if !reqCert[0].Equal(userCert) {
			http.Error(w, "User certificate does not match", http.StatusUnauthorized)
			return
		}

		log.Info().Msgf("Authenticated user: %s", user.Username)

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r)
	})
}
