package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

type Route interface {
	http.Handler

	// Pattern reports the path at which this is registered.
	Pattern() string
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

func NewRouter(routes []Route, logger *zerolog.Logger) *mux.Router {
	router := mux.NewRouter()
	for _, route := range routes {
		logger.Info().Msgf("Registering route: ", route.Pattern())
		router.Handle(route.Pattern(), route)
	}

	return router
}
