package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const HabitatAPIPort = "3000"

func NewAPIServer(lc fx.Lifecycle, router *mux.Router, logger *zerolog.Logger, proxyRules reverse_proxy.RuleSet) *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf(":%s", HabitatAPIPort), Handler: router}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			url, err := url.Parse("http://localhost:3000")
			if err != nil {
				return fmt.Errorf("Error parsing URL: %s", err)
			}
			proxyRules.Add("Habitat API", &reverse_proxy.RedirectRule{
				ForwardLocation: url,
				Matcher:         "/habitat/api",
			})
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			logger.Info().Msgf("Starting Habitat API server at %s", srv.Addr)
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
	return srv
}
