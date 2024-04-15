package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

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
			tlsConfig, err := nodeConfig.TLSConfig()
			if err != nil {
				return err
			}
			srv.TLSConfig = tlsConfig

			// Start the server
			url, err := url.Parse("https://localhost:3000")
			if err != nil {
				return fmt.Errorf("error parsing URL: %s", err)
			}
			err = proxyRules.Add("Habitat API", &reverse_proxy.RedirectRule{
				ForwardLocation: url,
				Matcher:         "/habitat/api",
			})
			if err != nil {
				return fmt.Errorf("error adding proxy rule: %s", err)
			}

			logger.Info().Msgf("Starting Habitat API server at %s", srv.Addr)
			go func() {
				err := srv.ListenAndServeTLS(nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath())
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
