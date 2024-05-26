package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/docker"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/hdbms"
	"github.com/eagraf/habitat-new/internal/node/logging"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
	"github.com/eagraf/habitat-new/internal/node/processes"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"golang.org/x/sync/errgroup"
)

var log = logging.NewLogger()

func main() {
	nodeConfig, err := config.NewNodeConfig()
	if err != nil {
		log.Fatal().Err(err)
	}

	hdbPublisher := pubsub.NewSimplePublisher[hdb.StateUpdate]()
	db, dbClose, err := hdbms.NewHabitatDB(log, hdbPublisher, nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer dbClose()

	nodeCtrl, err := controller.NewNodeController(db.Manager, nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
	}

	dockerDriver, err := docker.NewDockerDriver()
	if err != nil {
		log.Fatal().Err(err)
	}

	stateLogger := hdbms.NewStateUpdateLogger(log)
	appLifecycleSubscriber, err := package_manager.NewAppLifecycleSubscriber(dockerDriver.PackageManager, nodeCtrl)
	if err != nil {
		log.Fatal().Err(err)
	}

	pm := processes.NewProcessManager([]processes.ProcessDriver{dockerDriver.ProcessDriver})
	pmSub, err := processes.NewProcessManagerStateUpdateSubscriber(pm, nodeCtrl)
	if err != nil {
		log.Fatal().Err(err)
	}

	stateUpdates := pubsub.NewSimpleChannel([]pubsub.Publisher[hdb.StateUpdate]{hdbPublisher}, []pubsub.Subscriber[hdb.StateUpdate]{stateLogger, appLifecycleSubscriber, pmSub})
	go func() {
		err := stateUpdates.Listen()
		if err != nil {
			log.Fatal().Err(err).Msgf("unrecoverable error listening to channel")
		}
	}()

	err = nodeCtrl.InitializeNodeDB()
	if err != nil {
		log.Fatal().Err(err)
	}
	// ctx.Done() returns when SIGINT is called or cancel() is called.
	// calling cancel() unregisters the signal trapping.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	// egCtx is cancelled if any function called with eg.Go() returns an error.
	eg, egCtx := errgroup.WithContext(ctx)
	proxy := reverse_proxy.NewProxyServer(log, nodeConfig)
	tlsConfig, err := nodeConfig.TLSConfig()
	if err != nil {
		log.Fatal().Err(err)
	}
	proxyServer := &http.Server{
		Addr:      constants.DefaultPortReverseProxy,
		TLSConfig: tlsConfig,
		Handler:   proxy,
	}

	eg.Go(serveFn(proxyServer, "proxy-server"))

	routes := []api.Route{
		api.NewVersionHandler(),
		controller.NewGetNodeRoute(db.Manager),
		controller.NewAddUserRoute(nodeCtrl),
		controller.NewInstallAppRoute(nodeCtrl),
		controller.NewStartProcessHandler(nodeCtrl),
		controller.NewMigrationRoute(nodeCtrl),
	}

	router := api.NewRouter(routes, log, nodeCtrl, nodeConfig)
	apiServer := &http.Server{
		Addr:      fmt.Sprintf(":%s", constants.DefaultPortHabitatAPI),
		TLSConfig: tlsConfig,
		Handler:   router,
	}
	url, err := url.Parse(fmt.Sprintf("http://localhost:%s", constants.DefaultPortHabitatAPI))
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error parsing Habitat API URL: %v", err))
	}
	err = proxy.Rules.Add("Habitat API", &reverse_proxy.RedirectRule{
		ForwardLocation: url,
		Matcher:         "/habitat/api",
	})
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error adding Habitat API proxy rule: %v", err))
	}
	eg.Go(serveFn(apiServer, "api-server"))

	// Wait for either os.Interrupt which triggers ctx.Done()
	// Or one of the servers to error, which triggers egCtx.Done()
	select {
	case <-egCtx.Done():
		log.Err(fmt.Errorf("sub-service errored: shutting down Habitat %v", egCtx.Err()))
		cancel()
	case <-ctx.Done():
		log.Info().Msg("Interrupt signal received; gracefully closing Habitat")
	}

	// Shutdown the API server
	err = apiServer.Shutdown(context.Background())
	if err != nil {
		log.Err(fmt.Errorf("error on api-server shutdown: %v", err))
	}

	// Shutdown the proxy server
	err = proxyServer.Shutdown(context.Background())
	if err != nil {
		log.Err(fmt.Errorf("error on proxy-server shutdown: %v", err))
	}

	// Wait for the go-routines to finish
	err = eg.Wait()
	if err != nil {
		log.Err(fmt.Errorf("received error on eg.Wait(): %v", err))
	}
}

// config provided to http.Server.ListenAndServeTLS()
type tlsConfig struct {
	certFile string
	keyFile  string
}

// serverOptions provide optional config for an http.Server passed to serveFn()
type serverOption struct {
	tlsConfig *tlsConfig
}

// conventional way to supply arbitrary and optional arguments as options to a function.
type option func(*serverOption)

// WithTLSConfig provides serverOption with tlsConfig
func WithTLSConfig(certFile string, keyFile string) option {
	return func(so *serverOption) {
		so.tlsConfig = &tlsConfig{
			certFile: certFile,
			keyFile:  keyFile,
		}
	}
}

// serveFn takes in an http.Server and additional config and returns a callback that can be run in a separate go-routine.
func serveFn(srv *http.Server, name string, opts ...option) func() error {
	options := &serverOption{}
	for _, o := range opts {
		o(options)
	}
	return func() error {
		var err error
		if srv.TLSConfig != nil && options.tlsConfig != nil {
			log.Info().Msgf("Starting Habitat server[%s] at %s over TLS", name, srv.Addr)
			err = srv.ListenAndServeTLS(options.tlsConfig.certFile, options.tlsConfig.keyFile)
		} else {
			log.Warn().Msgf("No TSL config found: starting Habitat server[%s] at %s without TLS enabled", name, srv.Addr)
			err = srv.ListenAndServe()
		}
		if err != http.ErrServerClosed {
			log.Err(fmt.Errorf("a Habitat server[%s] closed with abnormal error: %v", name, err))
			return err
		}
		return nil
	}
}
