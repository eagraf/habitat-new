package reverse_proxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func NewProcessProxyRuleStateUpdateSubscriber(ruleSet RuleSet) (*hdb.IdempotentStateUpdateSubscriber, error) {
	return hdb.NewIdempotentStateUpdateSubscriber(
		"ProcessProxyRulesSubscriber",
		node.SchemaName,
		[]hdb.IdempotentStateUpdateExecutor{
			&ProcessProxyRulesExecutor{
				RuleSet: ruleSet,
			},
		},
		&ReverseProxyRestorer{
			ruleSet: ruleSet,
		},
	)
}

type ProxyServer struct {
	logger     *zerolog.Logger
	server     *http.Server
	nodeConfig *config.NodeConfig
	Rules      RuleSet
}

func NewProxyServer(ctx context.Context, logger *zerolog.Logger, config *config.NodeConfig) (*ProxyServer, func()) {
	srv := &ProxyServer{
		logger:     logger,
		Rules:      make(RuleSet),
		nodeConfig: config,
	}

	listenAddr := fmt.Sprintf(":%s", constants.DefaultPortReverseProxy)
	logger.Info().Msgf("Starting Habitat reverse proxy server at %s", listenAddr)
	go func() {
		err := srv.Start(listenAddr)
		log.Fatal().Err(err).Msg("reverse proxy server failed")
	}()

	return srv, func() {
		err := srv.server.Shutdown(ctx)
		if err != nil {
			log.Err(err)
		}
	}
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

func (s *ProxyServer) Start(addr string) error {
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s,
	}

	tlsConfig, err := s.nodeConfig.TLSConfig()
	if err != nil {
		return err
	}
	httpServer.TLSConfig = tlsConfig

	s.server = httpServer
	err = httpServer.ListenAndServeTLS(s.nodeConfig.NodeCertPath(), s.nodeConfig.NodeKeyPath())
	log.Fatal().Err(err).Msg("reverse proxy server failed")
	return nil
}
