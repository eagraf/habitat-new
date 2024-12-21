package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/appstore"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/drivers/docker"
	"github.com/eagraf/habitat-new/internal/node/drivers/web"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/hdbms"
	"github.com/eagraf/habitat-new/internal/node/logging"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
	"github.com/eagraf/habitat-new/internal/node/processes"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/eagraf/habitat-new/internal/node/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

func main() {
	nodeConfig, err := config.NewNodeConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("error loading node config")
	}

	logger := logging.NewLogger()
	zerolog.SetGlobalLevel(nodeConfig.LogLevel())

	hdbPublisher := pubsub.NewSimplePublisher[hdb.StateUpdate]()
	db, dbClose, err := hdbms.NewHabitatDB(logger, hdbPublisher, nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating habitat db")
	}
	defer dbClose()

	nodeCtrl, err := controller.NewNodeController(db.Manager, nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating node controller")
	}

	// Initialize application drivers
	dockerDriver, err := docker.NewDriver()
	if err != nil {
		log.Fatal().Err(err).Msg("error creating docker driver")
	}

	webDriver, err := web.NewDriver(nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating web driver")
	}

	stateLogger := hdbms.NewStateUpdateLogger(logger)
	appLifecycleSubscriber, err := package_manager.NewAppLifecycleSubscriber(
		map[string]package_manager.PackageManager{
			constants.AppDriverDocker: dockerDriver.PackageManager,
			constants.AppDriverWeb:    webDriver.PackageManager,
		},
		nodeCtrl,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating app lifecycle subscriber")
	}

	pm := processes.NewProcessManager([]processes.ProcessDriver{dockerDriver.ProcessDriver, webDriver.ProcessDriver})
	pmSub, err := processes.NewProcessManagerStateUpdateSubscriber(pm, nodeCtrl)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating process manager state update subscriber")
	}

	// ctx.Done() returns when SIGINT is called or cancel() is called.
	// calling cancel() unregisters the signal trapping.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// egCtx is cancelled if any function called with eg.Go() returns an error.
	eg, egCtx := errgroup.WithContext(ctx)

	proxy := reverse_proxy.NewProxyServer(logger, nodeConfig)

	proxyRuleStateUpdateSubscriber, err := reverse_proxy.NewProcessProxyRuleSubscriber(
		proxy.RuleSet,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating proxy rule state update subscriber")
	}

	stateUpdates := pubsub.NewSimpleChannel(
		[]pubsub.Publisher[hdb.StateUpdate]{hdbPublisher},
		[]pubsub.Subscriber[hdb.StateUpdate]{
			stateLogger,
			appLifecycleSubscriber,
			pmSub,
			proxyRuleStateUpdateSubscriber,
		},
	)
	go func() {
		err := stateUpdates.Listen()
		if err != nil {
			log.Fatal().Err(err).Msg("unrecoverable error listening to channel")
		}
	}()

	err = nodeCtrl.InitializeNodeDB()
	if err != nil {
		log.Fatal().Err(err).Msg("error initializing node db")
	}

	// Set up the reverse proxy server
	tlsConfig, err := nodeConfig.TLSConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting tls config")
	}
	addr := fmt.Sprintf(":%s", nodeConfig.ReverseProxyPort())
	proxyServer := &http.Server{
		Addr:    addr,
		Handler: proxy,
	}
	ln, err := proxy.Listener(addr)
	if err != nil {
		log.Fatal().Err(err).Msg("error getting listener")
	}
	eg.Go(server.ServeFn(
		proxyServer,
		"proxy-server",
		server.WithTLSConfig(tlsConfig, nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath()),
		server.WithListener(ln),
	))

	// Set up the main API server
	// TODO: create a less tedious way to register all the routes in the future. It might be as simple
	// as having a dedicated file to list these, instead of putting them all in main.
	routes := []api.Route{
		// Node routes
		api.NewVersionHandler(),
		controller.NewGetNodeRoute(db.Manager),
		controller.NewLoginRoute(controller.NewPDSClient(nodeConfig.PDSAdminUsername(), nodeConfig.PDSAdminUsername())),
		controller.NewAddUserRoute(nodeCtrl),
		controller.NewInstallAppRoute(nodeCtrl),
		controller.NewStartProcessHandler(nodeCtrl),
		controller.NewMigrationRoute(nodeCtrl),

		// App store routes
		appstore.NewAvailableAppsRoute(nodeConfig),
	}

	router := api.NewRouter(routes, logger, nodeCtrl, nodeConfig)
	apiServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", constants.DefaultPortHabitatAPI),
		Handler: router,
	}
	eg.Go(
		server.ServeFn(
			apiServer,
			"api-server",
			server.WithTLSConfig(tlsConfig, nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath()),
		),
	)

	// Wait for either os.Interrupt which triggers ctx.Done()
	// Or one of the servers to error, which triggers egCtx.Done()
	select {
	case <-egCtx.Done():
		log.Err(egCtx.Err()).Msg("sub-service errored: shutting down Habitat")
	case <-ctx.Done():
		log.Info().Msg("Interrupt signal received; gracefully closing Habitat")
		stop()
	}

	// Shutdown the API server
	err = apiServer.Shutdown(context.Background())
	if err != nil {
		log.Err(err).Msg("error on api-server shutdown")
	}
	log.Info().Msg("Gracefully shutdown Habitat API server")

	// Shutdown the proxy server
	err = proxyServer.Shutdown(context.Background())
	if err != nil {
		log.Err(err).Msg("error on proxy-server shutdown")
	}
	log.Info().Msg("Gracefully shutdown Habitat proxy server")

	// Wait for the go-routines to finish
	err = eg.Wait()
	if err != nil {
		log.Err(err).Msg("received error on eg.Wait()")
	}
	log.Info().Msg("Finished!")
}
