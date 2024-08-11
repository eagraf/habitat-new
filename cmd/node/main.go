package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/signal"
	"syscall"

	"github.com/bluesky-social/indigo/carstore"
	"github.com/bluesky-social/indigo/pds"
	"github.com/bluesky-social/indigo/plc"
	"github.com/bluesky-social/indigo/util/cliutil"
	"github.com/eagraf/habitat-new/internal/frontend"
	"github.com/eagraf/habitat-new/internal/node/api"
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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	nodeConfig, err := config.NewNodeConfig()
	if err != nil {
		log.Fatal().Err(err)
	}

	logger := logging.NewLogger()
	zerolog.SetGlobalLevel(nodeConfig.LogLevel())

	hdbPublisher := pubsub.NewSimplePublisher[hdb.StateUpdate]()
	db, dbClose, err := hdbms.NewHabitatDB(logger, hdbPublisher, nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer dbClose()

	nodeCtrl, err := controller.NewNodeController(db.Manager, nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
	}

	// Initialize application drivers
	dockerDriver, err := docker.NewDriver()
	if err != nil {
		log.Fatal().Err(err)
	}

	webDriver, err := web.NewDriver(nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
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
		log.Fatal().Err(err)
	}

	pm := processes.NewProcessManager([]processes.ProcessDriver{dockerDriver.ProcessDriver, webDriver.ProcessDriver})
	pmSub, err := processes.NewProcessManagerStateUpdateSubscriber(pm, nodeCtrl)
	if err != nil {
		log.Fatal().Err(err)
	}

	// ctx.Done() returns when SIGINT is called or cancel() is called.
	// calling cancel() unregisters the signal trapping.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// egCtx is cancelled if any function called with eg.Go() returns an error.
	eg, egCtx := errgroup.WithContext(ctx)

	proxy := reverse_proxy.NewProxyServer(logger, nodeConfig)

	proxyRuleStateUpdateSubscriber, err := reverse_proxy.NewProcessProxyRuleStateUpdateSubscriber(
		proxy.RuleSet,
	)
	if err != nil {
		log.Fatal().Err(err)
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
			log.Fatal().Err(err).Msgf("unrecoverable error listening to channel")
		}
	}()

	err = nodeCtrl.InitializeNodeDB()
	if err != nil {
		log.Fatal().Err(err)
	}

	// Set up the reverse proxy server
	tlsConfig, err := nodeConfig.TLSConfig()
	if err != nil {
		log.Fatal().Err(err)
	}
	addr := fmt.Sprintf(":%s", nodeConfig.ReverseProxyPort())
	proxyServer := &http.Server{
		Addr:    addr,
		Handler: proxy,
	}
	ln, err := proxy.Listener(addr)
	if err != nil {
		log.Fatal().Err(err)
	}
	eg.Go(server.ServeFn(
		proxyServer,
		"proxy-server",
		server.WithTLSConfig(tlsConfig, nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath()),
		server.WithListener(ln),
	))

	// Set up the main API server
	routes := []api.Route{
		api.NewVersionHandler(),
		controller.NewGetNodeRoute(db.Manager),
		controller.NewLoginRoute(&controller.PDSClient{NodeConfig: nodeConfig}),
		controller.NewAddUserRoute(nodeCtrl),
		controller.NewInstallAppRoute(nodeCtrl),
		controller.NewStartProcessHandler(nodeCtrl),
		controller.NewMigrationRoute(nodeCtrl),
	}

	router := api.NewRouter(routes, logger, nodeCtrl, nodeConfig)
	apiServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", constants.DefaultPortHabitatAPI),
		Handler: router,
	}
	url, err := url.Parse(fmt.Sprintf("http://localhost:%s", constants.DefaultPortHabitatAPI))
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error parsing Habitat API URL: %v", err))
	}
	err = proxy.RuleSet.Add("Habitat API", &reverse_proxy.RedirectRule{
		ForwardLocation: url,
		Matcher:         "/habitat/api",
	})
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error adding Habitat API proxy rule: %v", err))
	}
	eg.Go(
		server.ServeFn(
			apiServer,
			"api-server",
			server.WithTLSConfig(tlsConfig, nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath()),
		),
	)

	pdsUrl, err := url.Parse(fmt.Sprintf("http://localhost:%s", constants.DefaultPortPds))
	pdsServer, err := newPds(pdsUrl.Host)
	proxy.Rules.Add("PDS", &reverse_proxy.RedirectRule{
		ForwardLocation: pdsUrl,
		Matcher:         "/pds",
	})
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error creating PDS server: %v", err))
	}
	eg.Go(
		func() error {
			return pdsServer.RunAPI(":4989")
		},
	)

	frontendProxyRule, err := frontend.NewFrontendProxyRule(nodeConfig)
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error getting frontend proxy rule: %v", err))
	}

	err = proxy.RuleSet.Add("Frontend", frontendProxyRule)
	if err != nil {
		log.Fatal().Err(fmt.Errorf("error adding frontend proxy rule: %v", err))
	}

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

func newPds(host string) (*pds.Server, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
	})
	if err != nil {
		return nil, err
	}

	csdb, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
	})
	if err != nil {
		return nil, err
	}
	cstore, err := carstore.NewCarStore(csdb, "/pds/carstore")
	if err != nil {
		return nil, err
	}

	key, err := cliutil.LoadKeyFromFile("/pds/pds.key")
	if err != nil {
		return nil, err
	}

	didr := plc.NewFakeDid(db)

	srv, err := pds.NewServer(
		db,
		cstore,
		key,
		".test",
		host,
		didr,
		[]byte("bd6df801372d7058e1ce472305d7fc2e"),
	)
	if err != nil {
		return nil, err
	}

	return srv, nil

}
