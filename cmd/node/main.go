package main

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/docker"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/hdbms"
	"github.com/eagraf/habitat-new/internal/node/hdb/state"
	"github.com/eagraf/habitat-new/internal/node/logging"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
	"github.com/eagraf/habitat-new/internal/node/pubsub"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func main() {
	fx.New(
		fx.WithLogger(func(logger *zerolog.Logger) fxevent.Logger {
			return &logging.FxEventLoggerWrapper{Logger: logger}
		}),
		fx.Provide(logging.NewLogger),
		fx.Provide(config.NewNodeConfig),
		fx.Provide(
			api.NewAPIServer,
			fx.Annotate(
				api.NewRouter,
				fx.ParamTags(`group:"routes"`),
			),
		),
		fx.Provide(
			api.AsRoute(api.NewVersionHandler),
			api.AsRoute(controller.NewGetNodeHandler),
			api.AsRoute(controller.NewAddUserHandler),
			api.AsRoute(controller.NewInstallAppHandler),
		),
		fx.Provide(
			reverse_proxy.NewProxyServer,
		),
		fx.Provide(
			fx.Annotate(
				docker.NewDockerDriver,
				fx.As(new(package_manager.PackageManager)),
			),
		),
		fx.Provide(
			fx.Annotate(
				pubsub.NewSimplePublisher[state.StateUpdate],
				fx.As(new(pubsub.Publisher[state.StateUpdate])),
				fx.ParamTags(`group:"state_update"`),
			),
		),
		fx.Provide(
			fx.Annotate(
				hdbms.NewStateUpdateLogger,
				fx.As(new(pubsub.Subscriber[state.StateUpdate])),
				fx.ResultTags(`group:"state_update"`),
			),
		),
		fx.Provide(
			fx.Annotate(
				package_manager.NewAppLifecycleSubscriber,
				fx.As(new(pubsub.Subscriber[state.StateUpdate])),
				fx.ResultTags(`group:"state_update"`),
			),
		),
		fx.Provide(
			fx.Annotate(
				hdbms.NewHabitatDB,
				fx.As(new(hdb.HDBManager)),
			),
		),
		fx.Provide(
			fx.Annotate(
				controller.NewNodeController,
				fx.As(new(controller.NodeController)),
			),
		),
		fx.Invoke(func(controller.NodeController) {}),
		fx.Invoke(func(*http.Server) {}),
	).Run()
}
