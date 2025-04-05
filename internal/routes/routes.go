package Routes

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/api"
)

// Helper package to easily retur
type Route struct {
	method  string
	pattern string
	fn      http.HandlerFunc
}

func newRoute(method, pattern string, fn http.HandlerFunc) api.Route {
	return &Route{
		method, pattern, fn,
	}
}

func (r *Route) Method() string {
	return r.method
}

func (r *Route) Pattern() string {
	return r.pattern
}

func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.fn(w, req)
}
