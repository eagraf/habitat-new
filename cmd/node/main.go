package main

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/api"
	"go.uber.org/fx"
)

func main() {
	fx.New(
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
		fx.Invoke(func(*http.Server) {}),
	).Run()
}
