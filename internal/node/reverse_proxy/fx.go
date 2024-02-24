package reverse_proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

func NewProxyServer(lc fx.Lifecycle, logger *zerolog.Logger) (*ProxyServer, RuleSet) {
	srv := &ProxyServer{
		logger: logger,
		Rules:  make(RuleSet),
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listenAddr := fmt.Sprintf(":%s", constants.DefaultPortReverseProxy)
			logger.Info().Msgf("Starting Habitat reverse proxy server at %s", listenAddr)
			go srv.Start(listenAddr)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.server.Shutdown(ctx)
		},
	})
	return srv, srv.Rules
}

type ProxyServer struct {
	logger *zerolog.Logger
	server *http.Server
	Rules  RuleSet
}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rule := range s.Rules {
		if rule.Match(r.URL) {
			rule.Handler().ServeHTTP(w, r)
			return
		}
	}
	// No rules matched
	w.WriteHeader(http.StatusNotFound)
}

func (s *ProxyServer) Start(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("reverse proxy server failed to start")
	}
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s,
	}
	s.server = httpServer
	log.Fatal().Err(httpServer.Serve(ln)).Msg("reverse proxy server failed")
}
