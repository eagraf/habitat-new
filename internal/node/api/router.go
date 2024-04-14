package api

import (
	"fmt"
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"tailscale.com/tsnet"
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

func NewRouter(routes []Route, logger *zerolog.Logger, nodeController controller.NodeController, nodConfig *config.NodeConfig, s *tsnet.Server) *mux.Router {
	router := mux.NewRouter()
	for _, route := range routes {
		logger.Info().Msgf("Registering route: %s", route.Pattern())
		router.Handle(route.Pattern(), route).Methods(route.Method())
	}

	/*
		authMiddleware := &authenticationMiddleware{
			nodeController: nodeController,
			nodeConfig:     nodConfig,
		}

		router.Use(authMiddleware.Middleware)
	*/
	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lc, err := s.LocalClient()
			if err != nil {
				logger.Fatal().Msgf("%v", err)
			}
			who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
			if err != nil {
				return
			}
			fmt.Println("logging middleware", who)
		})
	})
	return router
}
