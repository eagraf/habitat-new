package main

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/eagraf/habitat-new/internal/node/habitat_db"
	"github.com/eagraf/habitat-new/internal/node/logging"
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
			api.AsRoute(controller.NewPostUserHandler),
		),
		fx.Provide(
			reverse_proxy.NewProxyServer,
		),
		fx.Provide(
			habitat_db.NewHabitatDB,
		),
		fx.Provide(
			controller.NewNodeController,
		),
		fx.Invoke(func(*controller.NodeController) {}),
		fx.Invoke(func(*http.Server) {}),
	).Run()
}
