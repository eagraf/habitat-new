package controller

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/package_manager"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/pkg/errors"
)

type CtrlServer struct {
	inner *controller2
}

func NewCtrlServer(ctx context.Context, b *BaseNodeController, processManager process.ProcessManager, pkgManagers map[node.DriverType]package_manager.PackageManager, db hdb.Client) (*CtrlServer, error) {
	inner, err := newController2(ctx, processManager, pkgManagers, db)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing controller")
	}

	b.SetCtrl2(inner)

	state, err := inner.getNodeState()
	if err != nil {
		return nil, errors.Wrap(err, "error getting initial node state")
	}
	err = inner.restore(state)
	if err != nil {
		return nil, errors.Wrap(err, "error restoring controller to initial state")
	}

	return &CtrlServer{
		inner: inner,
	}, nil
}

type StartProcessRequest struct {
	AppInstallationID string `json:"app_id"`
}

func (s *CtrlServer) StartProcess(w http.ResponseWriter, r *http.Request) {
	var req StartProcessRequest
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

type StopProcessRequest struct {
	ProcessID node.ProcessID `json:"process_id"`
}

func (s *CtrlServer) StopProcess(w http.ResponseWriter, r *http.Request) {
	var req StopProcessRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.inner.stopProcess(req.ProcessID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *CtrlServer) ListProcesses(w http.ResponseWriter, r *http.Request) {
	procs, err := s.inner.processManager.ListRunningProcesses(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bytes, err := json.Marshal(procs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(bytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type InstallAppRequest struct {
	AppInstallation   *node.AppInstallation    `json:"app_installation" yaml:"app_installation"`
	ReverseProxyRules []*node.ReverseProxyRule `json:"reverse_proxy_rules" yaml:"reverse_proxy_rules"`
	StartAfterInstall bool
}

func (s *CtrlServer) InstallApp(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	var req InstallAppRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	appInstallation := req.AppInstallation

	err = s.inner.installApp(userID, appInstallation.Package, appInstallation.Version, appInstallation.Name, req.ReverseProxyRules, req.StartAfterInstall)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO validate request
	w.WriteHeader(http.StatusCreated)
}

func (s *CtrlServer) GetNodeState(w http.ResponseWriter, r *http.Request) {
	state, err := s.inner.getNodeState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bytes, err := json.Marshal(state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(bytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
		newRoute(http.MethodGet, "/node/processes/list", s.ListProcesses),
		newRoute(http.MethodPost, "/node/processes/start", s.StartProcess),
		newRoute(http.MethodPost, "/node/processes/stop", s.StopProcess),
		newRoute(http.MethodGet, "/node/state", s.GetNodeState),
		newRoute(http.MethodPost, "/node/users/{user_id}/apps", s.InstallApp),
	}
}
