package controller

import (
	"encoding/json"
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/pkg/errors"
)

type CtrlServer struct {
	inner *controller2
}

func NewCtrlServer(pm process.ProcessManager, db hdb.Client) (*CtrlServer, error) {
	inner, err := newController2(pm, db)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing controller")
	}

	return &CtrlServer{
		inner: inner,
	}, nil
}

type PostProcessRequest struct {
	AppInstallationID string `json:"app_id"`
}

func (s *CtrlServer) StartProcess(w http.ResponseWriter, r *http.Request) {
	var req PostProcessRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.inner.startProcess(req.AppInstallationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

type route struct {
	method  string
	pattern string
	fn      http.HandlerFunc
}

func newRoute(method, pattern string, fn http.HandlerFunc) *route {
	return &route{
		method, pattern, fn,
	}
}

func (r *route) Method() string {
	return r.method
}

func (r *route) Pattern() string {
	return r.pattern
}

func (r *route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.fn(w, req)
}

var _ api.Route = &route{}

func (s *CtrlServer) GetRoutes() []api.Route {
	return []api.Route{
		newRoute(http.MethodPost, "/node/processes", s.StartProcess),
	}
}
