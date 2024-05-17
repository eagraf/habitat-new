package reverse_proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
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

func NewProxyServer(logger *zerolog.Logger, config *config.NodeConfig) *ProxyServer {
	return &ProxyServer{
		logger:     logger,
		Rules:      make(RuleSet),
		nodeConfig: config,
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

func (s *ProxyServer) Start(addr string, tlsConfig *tls.Config) (func(), error) {
	httpServer := &http.Server{
		Addr:      addr,
		Handler:   s,
		TLSConfig: tlsConfig,
	}

	s.server = httpServer
	s.logger.Info().Msgf("Starting Habitat reverse proxy server at %s", addr)

	eg := &errgroup.Group{}
	eg.Go(func() error {
		var err error
		if tlsConfig != nil {
			err = httpServer.ListenAndServeTLS(s.nodeConfig.NodeCertPath(), s.nodeConfig.NodeKeyPath())
		} else {
			err = httpServer.ListenAndServe()
		}
		return err
	})
	return func() {
		err := s.server.Close()
		if err != nil {
			log.Error().Msg(fmt.Sprintf("error shutting down revere proxy server %v", err))
		}
		err = eg.Wait()
		if err != nil {
			log.Err(err)
		}
	}, nil
}
