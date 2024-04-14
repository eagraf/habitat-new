package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const CertificateDir = "/dev_certificates"

func NewAPIServer(lc fx.Lifecycle, router *mux.Router, logger *zerolog.Logger, proxyRules reverse_proxy.RuleSet, nodeConfig *config.NodeConfig) *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf(":%s", constants.DefaultPortHabitatAPI), Handler: router}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			/*
				tlsConfig, err := generateTLSConfig(nodeConfig)
				if err != nil {
					return err
				}
				srv.TLSConfig = tlsConfig
			*/

			srv.Addr = ":3000"
			// Start the server
			url, err := url.Parse("http://localhost:3000")
			if err != nil {
				return fmt.Errorf("Error parsing URL: %s", err)
			}
			err = proxyRules.Add("Habitat API", &reverse_proxy.RedirectRule{
				ForwardLocation: url,
				Matcher:         "/habitat/api",
			})
			if err != nil {
				return fmt.Errorf("Error adding proxy rule: %s", err)
			}

			logger.Info().Msgf("Starting Habitat API server at %s", srv.Addr)
			go func() {
				err := srv.ListenAndServe()
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

func generateTLSConfig(config *config.NodeConfig) (*tls.Config, error) {
	rootCertBytes, err := os.ReadFile(config.RootUserCertPath())
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCertBytes)

	return &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}
