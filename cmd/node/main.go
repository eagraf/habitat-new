package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bluesky-social/indigo/carstore"
	custom_pds "github.com/bluesky-social/indigo/pds"
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
		log.Fatal().Err(err).Msg("error setting up node config")
	}

	logger := logging.NewLogger()
	zerolog.SetGlobalLevel(nodeConfig.LogLevel())

	hdbPublisher := pubsub.NewSimplePublisher[hdb.StateUpdate]()
	db, dbClose, err := hdbms.NewHabitatDB(logger, hdbPublisher, nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating habitat DB")
	}
	defer dbClose()

	nodeCtrl, err := controller.NewNodeController(db.Manager, nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating node controller")
	}

	// Initialize application drivers
	dockerDriver, err := docker.NewDriver()
	if err != nil {
		log.Fatal().Err(err).Msg("error creating Docker driver")
	}

	webDriver, err := web.NewDriver(nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating Web driver")
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
		log.Fatal().Err(err).Msg("error creating lifecycle subscriber")
	}

	pm := processes.NewProcessManager([]processes.ProcessDriver{dockerDriver.ProcessDriver, webDriver.ProcessDriver})
	pmSub, err := processes.NewProcessManagerStateUpdateSubscriber(pm, nodeCtrl)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating process manager subscriber")
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
		log.Fatal().Err(err).Msg("error creating proxy update subscriber")
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
		log.Fatal().Err(err).Msg("error initializing node DB")
	}

	// Set up the reverse proxy server
	tlsConfig, err := nodeConfig.TLSConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("error getting TLS config")
	}
	addr := fmt.Sprintf(":%s", nodeConfig.ReverseProxyPort())
	proxyServer := &http.Server{
		Addr:    addr,
		Handler: proxy,
	}
	ln, err := proxy.Listener(addr)
	if err != nil {
		log.Fatal().Err(err).Msg("error getting proxy listner")
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
		log.Fatal().Err(err).Msg("error parsing Habitat API URL")
	}
	err = proxy.RuleSet.Add("Habitat API", &reverse_proxy.RedirectRule{
		ForwardLocation: url,
		Matcher:         "/habitat/api",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error adding Habitat API proxy rule")
	}
	eg.Go(
		server.ServeFn(
			apiServer,
			"api-server",
			server.WithTLSConfig(tlsConfig, nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath()),
		),
	)

	pdsUrl, err := url.Parse(fmt.Sprintf("http://localhost:%s", constants.DefaultPortPds))
	if err != nil {
		log.Fatal().Err(err).Msg("error parsing pds URL")
	}
	pdsServer, err := newPds(nodeConfig.PDSPath(), nodeConfig.Domain())
	if err != nil {
		log.Fatal().Err(err).Msg("error creating PDS")
	}
	err = proxy.RuleSet.Add("PDS", &reverse_proxy.RedirectRule{
		ForwardLocation: pdsUrl,
		Matcher:         "/pds",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error adding PDS proxy")
	}
	pdsXRPCUrl, err := url.Parse(fmt.Sprintf("http://localhost:%s/xrpc", constants.DefaultPortPds))
	if err != nil {
		log.Fatal().Err(err).Msg("error parsing PDS - xrpc URL")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("error adding PDS - xrpc proxy rule")
	}
	err = proxy.RuleSet.Add("PDS - xrpc", &reverse_proxy.RedirectRule{
		ForwardLocation: pdsXRPCUrl,
		Matcher:         "/xrpc",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error adding PDS - xrpc proxy rule")
	}

	eg.Go(func() error {
		return pdsServer.RunAPI(pdsUrl.Hostname() + ":" + pdsUrl.Port())
	})

	frontendProxyRule, err := frontend.NewFrontendProxyRule(nodeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("error getting frontend proxy rule")
	}

	err = proxy.RuleSet.Add("Frontend", frontendProxyRule)
	if err != nil {
		log.Fatal().Err(err).Msg("error adding frontend proxy rule")
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

func newPds(path, domain string) (*custom_pds.Server, error) {
	err := os.Mkdir(path, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(path, "pds.db")), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
	})
	if err != nil {
		return nil, err
	}

	csdb, err := gorm.Open(sqlite.Open(filepath.Join(path, "carstore.db")), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
	})
	if err != nil {
		return nil, err
	}
	cstore, err := carstore.NewCarStore(csdb, filepath.Join(path, "carstore"))
	if err != nil {
		return nil, err
	}

	keyPath := filepath.Join(path, "pds.key")
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		err = cliutil.GenerateKeyToFile(keyPath)
		if err != nil {
			return nil, err
		}
	}
	key, err := cliutil.LoadKeyFromFile(keyPath)
	if err != nil {
		return nil, err
	}

	tsDomain := "habitat-dev.taile529e.ts.net"
	srv, err := custom_pds.NewServer(
		db,
		cstore,
		key,
		fmt.Sprintf(".%s", tsDomain),
		tsDomain,
		plc.NewFakeDid(db),
		[]byte("test signing key"),
	)
	if err != nil {
		return nil, err
	}

	return srv, nil
}
