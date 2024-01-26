package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const HabitatAPIPort = "3000"
const CertificateDir = "/dev_certificates"

func NewAPIServer(lc fx.Lifecycle, router *mux.Router, logger *zerolog.Logger, proxyRules reverse_proxy.RuleSet) *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf(":%s", HabitatAPIPort), Handler: router}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// TODO client and server should use different certificates
			serverCertPath := filepath.Join(CertificateDir, "cert.pem")
			serverKeyPath := filepath.Join(CertificateDir, "key.pem")

			// Set up the certificate pool
			defaultUserCert, err := os.ReadFile(filepath.Join(CertificateDir, "cert.pem"))
			if err != nil {
				return err
			}

			if err != nil {
				return err
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(defaultUserCert)

			// Create the TLS Config with the CA pool and enable Client certificate validation
			srv.TLSConfig = &tls.Config{
				ClientCAs:  caCertPool,
				ClientAuth: tls.RequireAndVerifyClientCert,
			}

			// Start the server
			url, err := url.Parse("http://localhost:3000")
			if err != nil {
				return fmt.Errorf("Error parsing URL: %s", err)
			}
			proxyRules.Add("Habitat API", &reverse_proxy.RedirectRule{
				ForwardLocation: url,
				Matcher:         "/habitat/api",
			})

			logger.Info().Msgf("Starting Habitat API server at %s", srv.Addr)
			go func() {
				err := srv.ListenAndServeTLS(serverCertPath, serverKeyPath)
				logger.Fatal().Msgf("Habitat API server error: %s", err)
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
	return srv
}
