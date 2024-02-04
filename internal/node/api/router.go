package api

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

type Route interface {
	http.Handler

	// Pattern reports the path at which this is registered.
	Pattern() string
	Method() string
}

// AsRoute annotates the given constructor to state that
// it provides a route to the "routes" group.
func AsRoute(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(Route)),
		fx.ResultTags(`group:"routes"`),
	)
}

func NewRouter(routes []Route, logger *zerolog.Logger, nodeController controller.NodeController, nodConfig *config.NodeConfig) *mux.Router {
	router := mux.NewRouter()
	for _, route := range routes {
		logger.Info().Msgf("Registering route: %s", route.Pattern())
		router.Handle(route.Pattern(), route).Methods(route.Method())
	}

	authMiddleware := &authenticationMiddleware{
		nodeController: nodeController,
		nodeConfig:     nodConfig,
	}

	router.Use(authMiddleware.Middleware)

	return router
}
