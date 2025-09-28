package api

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/rs/zerolog"
)

type Route interface {
	http.Handler

	// Pattern reports the path at which this is registered.
	Pattern() string
	Methods() []string
}

type processedRoute struct {
	Route
}

func processRoute(route Route) processedRoute {
	return processedRoute{route}
}

func (p processedRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !slices.Contains(p.Route.Methods(), r.Method) {
		http.Error(
			w,
			fmt.Sprintf("invalid method, require %s", p.Route.Methods()),
			http.StatusMethodNotAllowed,
		)
		return
	}
	p.ServeHTTP(w, r)
}

func NewRouter(
	routes []Route,
	logger *zerolog.Logger,
) http.Handler {
	router := http.NewServeMux()
	for _, route := range routes {
		logger.Info().Msgf("Registering route: %s", route.Pattern())
		router.Handle(route.Pattern(), processRoute(route))
	}

	return router
}

// Helper package to easily return structured routes given basic info.
type basicRoute struct {
	method  string
	pattern string
	fn      http.HandlerFunc
}

func NewBasicRoute(method, pattern string, fn http.HandlerFunc) Route {
	return &basicRoute{
		method, pattern, fn,
	}
}

func (r *basicRoute) Methods() []string {
	return []string{r.method}
}

func (r *basicRoute) Pattern() string {
	return r.pattern
}

func (r *basicRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.fn(w, req)
}
