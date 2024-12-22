package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/core/state/node"
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
	"github.com/google/uuid"

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
	db, dbClose, err := hdbms.NewHabitatDB(logger, hdbPublisher, nodeConfig.HDBPath())
	if err != nil {
		log.Fatal().Err(err).Msg("error creating habitat db")
	}
	defer dbClose()

	pdsClient := controller.NewPDSClient(nodeConfig.PDSAdminUsername(), nodeConfig.PDSAdminPassword())
	nodeCtrl, err := controller.NewNodeController(db.Manager, pdsClient)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating node controller")
	}

	// Initialize application drivers
	dockerDriver, err := docker.NewDriver()
	if err != nil {
		log.Fatal().Err(err).Msg("error creating docker driver")
	}

	webDriver, err := web.NewDriver(nodeConfig.WebBundlePath())
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

	proxy := reverse_proxy.NewProxyServer(logger, nodeConfig.WebBundlePath())
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

	initState, err := generateInitState(nodeConfig.RootUserCertB64())
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate initial node state")
	}

	// Generate the list of default proxy rules to have available when the node first comes up
	proxyRules, err := generateDefaultReverseProxyRules(nodeConfig.FrontendDev())
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate proxy rules")
	}

	// Generate the list of apps to have installed and started when the node first comes up
	pdsAppConfig := generatePDSAppConfig(nodeConfig)
	defaultApps := append([]*types.PostAppRequest{
		pdsAppConfig,
	}, nodeConfig.DefaultApps()...)
	log.Info().Msgf("configDefaultApps: %v", defaultApps)

	initialTransitions, err := initTranstitions(initState, defaultApps, proxyRules)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to generate proxy rules")
	}

	err = nodeCtrl.InitializeNodeDB(initialTransitions)
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

	var ln net.Listener
	// If TS_AUTHKEY is set, create a tsnet listener. Otherwise, create a normal tcp listener.
	if nodeConfig.TailscaleAuthkey() == "" {
		ln, err = proxy.Listener(addr)
	} else {
		ln, err = proxy.TailscaleListener(addr, nodeConfig.Hostname(), nodeConfig.TailScaleStatePath(), nodeConfig.TailScaleFunnelEnabled())
	}

	if err != nil {
		log.Fatal().Err(err).Msg("error creating reverse proxy listener")
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
		controller.NewLoginRoute(pdsClient),
		controller.NewAddUserRoute(nodeCtrl),
		controller.NewInstallAppRoute(nodeCtrl),
		controller.NewStartProcessHandler(nodeCtrl),
		controller.NewMigrationRoute(nodeCtrl),
	}
	if nodeConfig.Environment() == constants.EnvironmentDev {
		// App store is unimplemented in production
		routes = append(routes, appstore.NewAvailableAppsRoute(nodeConfig.HabitatPath()))
	}

	router := api.NewRouter(routes, logger, nodeCtrl, nodeConfig.UseTLS(), nodeConfig.RootUserCert)
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

func generatePDSAppConfig(nodeConfig *config.NodeConfig) *types.PostAppRequest {
	pdsMountDir := filepath.Join(nodeConfig.HabitatAppPath(), "pds")

	// TODO @eagraf - unhardcode as much of this as possible
	return &types.PostAppRequest{
		AppInstallation: &node.AppInstallation{
			Name:    "pds",
			Version: "1",
			UserID:  constants.RootUserID,
			Package: node.Package{
				Driver: "docker",
				DriverConfig: map[string]interface{}{
					"env": []string{
						fmt.Sprintf("PDS_HOSTNAME=%s", nodeConfig.Domain()),
						"PDS_DATA_DIRECTORY=/pds",
						"PDS_BLOBSTORE_DISK_LOCATION=/pds/blocks",
						"PDS_PLC_ROTATION_KEY_K256_PRIVATE_KEY_HEX=5290bb1866a03fb23b09a6ffd64d21f6a4ebf624eaa301930eeb81740699239c",
						"PDS_JWT_SECRET=bd6df801372d7058e1ce472305d7fc2e",
						"PDS_ADMIN_PASSWORD=password",
						"PDS_BSKY_APP_VIEW_URL=https://api.bsky.app",
						"PDS_BSKY_APP_VIEW_DID=did:web:api.bsky.app",
						"PDS_REPORT_SERVICE_URL=https://mod.bsky.app",
						"PDS_INVITE_REQUIRED=false",
						"PDS_REPORT_SERVICE_DID=did:plc:ar7c4by46qjdydhdevvrndac",
						"PDS_CRAWLERS=https://bsky.network",
						"DEBUG=t",
					},
					"mounts": []mount.Mount{
						{
							Type:   "bind",
							Source: pdsMountDir,
							Target: "/pds",
						},
					},
					"exposed_ports": []string{"5001"},
					"port_bindings": map[nat.Port][]nat.PortBinding{
						"3000/tcp": {
							{
								HostIP:   "0.0.0.0",
								HostPort: "5001",
							},
						},
					},
				},
				RegistryURLBase:    "registry.hub.docker.com",
				RegistryPackageID:  "ethangraf/pds",
				RegistryPackageTag: "latest",
			},
		},
		ReverseProxyRules: []*node.ReverseProxyRule{
			{
				Type:    "redirect",
				Matcher: "/xrpc",
				Target:  "http://host.docker.internal:5001/xrpc",
			},
		},
	}
}

func generateDefaultReverseProxyRules(frontendDev bool) ([]*node.ReverseProxyRule, error) {
	apiURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", constants.DefaultPortHabitatAPI))
	if err != nil {
		return nil, err
	}

	frontendRule := &node.ReverseProxyRule{
		ID:      "default-rule-frontend",
		Matcher: "", // Root matcher
	}
	if frontendDev {
		// In development mode, we run the frontend in a separate docker container with hot-reloading.
		// As a result, all frontend requests must be forwarde to the frontend container.
		frontendRule.Type = node.ProxyRuleRedirect
		frontendRule.Target = "http://habitat_frontend:8000/"
	} else {
		// In production mode, we embed the frontend into the node binary. That way, we can serve
		// the frontend without needing to set it up on the host machine.
		// TODO @eagraf - evaluate the performance implications of this.
		frontendRule.Type = node.ProxyRuleEmbeddedFrontend
	}

	return []*node.ReverseProxyRule{
		{
			ID:      "default-rule-api",
			Type:    node.ProxyRuleRedirect,
			Matcher: "/habitat/api",
			Target:  apiURL.String(),
		},
		frontendRule,
	}, nil
}

func generateInitState(rootUserCert string) (*node.State, error) {
	// TODO this is basically a placeholder until we actually have a way of generating
	// the certificate for the node.
	nodeUUID := uuid.New().String()
	rootCert := rootUserCert

	initState, err := node.GetEmptyStateForVersion(node.LatestVersion)
	if err != nil {
		return nil, err
	}

	initState.NodeID = nodeUUID
	initState.Users[constants.RootUserID] = &node.User{
		ID:          constants.RootUserID,
		Username:    constants.RootUsername,
		Certificate: rootCert,
	}

	return initState, nil
}

func initTranstitions(initState *node.State, startApps []*types.PostAppRequest, proxyRules []*node.ReverseProxyRule) ([]hdb.Transition, error) {
	// A list of transitions to apply when the node starts up for the first time.
	transitions := []hdb.Transition{
		&node.InitalizationTransition{
			InitState: initState,
		},
	}

	for _, rule := range proxyRules {
		transitions = append(transitions, &node.AddReverseProxyRuleTransition{
			Rule: rule,
		})
	}

	for _, app := range startApps {
		transitions = append(transitions, &node.StartInstallationTransition{
			UserID:                 constants.RootUserID,
			AppInstallation:        app.AppInstallation,
			NewProxyRules:          app.ReverseProxyRules,
			StartAfterInstallation: true,
		})
	}

	return transitions, nil
}
