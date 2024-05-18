package api

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type Route interface {
	http.Handler

	// Pattern reports the path at which this is registered.
	Pattern() string
	Method() string
}

func NewRouter(routes []Route, logger *zerolog.Logger, nodeController controller.NodeController, nodeConfig *config.NodeConfig) *mux.Router {
	router := mux.NewRouter()
	for _, route := range routes {
		logger.Info().Msgf("Registering route: %s", route.Pattern())
		router.Handle(route.Pattern(), route).Methods(route.Method())
	}

	authMiddleware := &authenticationMiddleware{
		nodeController: nodeController,
		nodeConfig:     nodeConfig,
	}

	router.Use(authMiddleware.Middleware)

	return router
}
