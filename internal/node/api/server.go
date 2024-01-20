package api

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const HabitatAPIPort = "3000"

func NewAPIServer(lc fx.Lifecycle, router *mux.Router, logger *zerolog.Logger) *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf(":%s", HabitatAPIPort), Handler: router}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			logger.Info().Msgf("Starting Habitat API server at", srv.Addr)
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
	return srv
}
