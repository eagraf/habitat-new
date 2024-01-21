package main

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/api"
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
		fx.Provide(
			api.NewAPIServer,
			fx.Annotate(
				api.NewRouter,
				fx.ParamTags(`group:"routes"`),
			),
		),
		fx.Provide(
			api.AsRoute(api.NewVersionHandler),
		),
		fx.Provide(
			reverse_proxy.NewProxyServer,
		),
		fx.Invoke(func(*http.Server) {}),
	).Run()
}
