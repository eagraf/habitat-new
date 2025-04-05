package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type CtrlServer struct {
	inner   *Controller2
	pdsHost string
}

func NewCtrlServer(
	ctx context.Context,
	b *BaseNodeController,
	pdsHost string,
	inner *Controller2,
	state *node.State,
) (*CtrlServer, error) {
	b.SetCtrl2(inner)
	err := inner.restore(state)
	if err != nil {
		return nil, errors.Wrap(err, "error restoring controller to initial state")
	}

	return &CtrlServer{
		inner:   inner,
		pdsHost: pdsHost,
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

	if _, err := w.Write(bytes); err != nil {
		log.Err(err).Msgf("error sending response in for ListProcesses request")
	}
}

type InstallAppRequest struct {
	AppInstallation   *node.AppInstallation    `json:"app_installation" yaml:"app_installation"`
	ReverseProxyRules []*node.ReverseProxyRule `json:"reverse_proxy_rules" yaml:"reverse_proxy_rules"`
	StartAfterInstall bool
}

func (s *CtrlServer) InstallApp(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	// TODO: authenticate user

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

type UninstallAppRequest struct {
	AppID string `json:"app_id" yaml:"app_installation"`
}

func (s *CtrlServer) UninstallApp(w http.ResponseWriter, r *http.Request) {
	var req UninstallAppRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.inner.uninstallApp(req.AppID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO validate request
	w.WriteHeader(http.StatusOK)
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

	if _, err := w.Write(bytes); err != nil {
		log.Err(err).Msgf("error sending response in for GetNodeState request")
	}
}

type PutRecordRequest struct {
	Input   *agnostic.RepoPutRecord_Input
	Encrypt bool `json:"encrypt"`
}

func (s *CtrlServer) PutRecord(cli *xrpc.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PutRecordRequest
		slurp, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(slurp, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println(req)

		out, err := s.inner.putRecord(r.Context(), cli, req.Input, req.Encrypt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		slurp, err = json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := w.Write(slurp); err != nil {
			log.Err(err).Msgf("error sending response for PutRecord request")
		}
	}
}

func (s *CtrlServer) GetRecord(cli *xrpc.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		cid := u.Query().Get("cid")
		collection := u.Query().Get("collection")
		did := u.Query().Get("did")
		rkey := u.Query().Get("rkey")

		out, err := s.inner.getRecord(r.Context(), cli, cid, collection, did, rkey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slurp, err := json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(slurp); err != nil {
			log.Err(err).Msgf("error sending response for GetRecord request")
		}
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

func (s *CtrlServer) pdsAuthMiddleware(next func(c *xrpc.Client) http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		c := &xrpc.Client{
			Host: fmt.Sprintf("http://%s", s.pdsHost),
			Auth: &xrpc.AuthInfo{
				AccessJwt: bearer,
			},
		}
		next(c).ServeHTTP(w, r)
	})
}

func (s *CtrlServer) GetRoutes() []api.Route {
	return []api.Route{
		newRoute(http.MethodGet, "/node/processes/list", s.ListProcesses),
		newRoute(http.MethodPost, "/node/processes/start", s.StartProcess),
		newRoute(http.MethodPost, "/node/processes/stop", s.StopProcess),
		newRoute(http.MethodGet, "/node/state", s.GetNodeState),
		newRoute(http.MethodPost, "/node/apps/{user_id}/install", s.InstallApp),
		newRoute(http.MethodPost, "/node/apps/uninstall", s.UninstallApp),
		newRoute(http.MethodPost, "/xrpc/com.habitat.putRecord", s.pdsAuthMiddleware(s.PutRecord)),
		newRoute(http.MethodGet, "/xrpc/com.habitat.getRecord", s.pdsAuthMiddleware(s.GetRecord)),
	}
}
