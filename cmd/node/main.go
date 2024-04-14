package main

import (
	"context"
	"fmt"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/docker"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/hdbms"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
	"github.com/eagraf/habitat-new/internal/node/processes"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
)

func main() {
	ctx := context.Background()
	log := logging.NewLogger()

	nodeConfig, err := config.NewNodeConfig()
	if err != nil {
		log.Fatal().Err(err)
	}

	hdbPublisher := pubsub.NewSimplePublisher[hdb.StateUpdate]()
	db, dbClose, err := hdbms.NewHabitatDB(log, nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer dbClose()

	nodeCtrl := controller.NewNodeController(db.Manager, nodeConfig)

	routes := []api.Route{
		api.NewVersionHandler(),
		controller.NewGetNodeRoute(db.Manager),
		controller.NewAddUserRoute(nodeCtrl),
		controller.NewInstallAppRoute(nodeCtrl),
		controller.NewStartProcessHandler(nodeCtrl),
		controller.NewMigrationRoute(nodeCtrl),
	}

	router := api.NewRouter(routes, log, nodeCtrl, nodeConfig)
	proxy, ruleset, proxyClose := reverse_proxy.NewProxyServer(ctx, log)
	listenAddr := fmt.Sprintf(":%s", constants.DefaultPortReverseProxy)
	log.Info().Msgf("Starting Habitat reverse proxy server at %s", listenAddr)
	go proxy.Start(listenAddr)
	defer proxyClose()

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

	go pubsub.NewSimpleChannel([]pubsub.Publisher[hdb.StateUpdate]{hdbPublisher}, []pubsub.Subscriber[hdb.StateUpdate]{stateLogger, appLifecycleSubscriber, pmSub})

	server, err := api.NewAPIServer(ctx, router, log, ruleset, nodeConfig)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer server.Close()
	err = server.ListenAndServeTLS(nodeConfig.NodeCertPath(), nodeConfig.NodeKeyPath())
	log.Fatal().Msgf("Habitat API server error: %s", err)
}
