package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/bluesky-social/indigo/api/atproto"
	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type CtrlServer struct {
	inner *Controller2
}

func NewCtrlServer(
	ctx context.Context,
	inner *Controller2,
	state *node.State,
) (*CtrlServer, error) {
	err := inner.restore(state)
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

func (s *CtrlServer) AddUser(w http.ResponseWriter, r *http.Request) {
	var req atproto.ServerCreateAccount_Input
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

	out, err := s.inner.addUser(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(bytes); err != nil {
		log.Err(err).Msgf("error sending response in for AddUser request")
	}
}

type MigrateRequest struct {
	TargetVersion string `json:"target_version"`
}

func (s *CtrlServer) MigrateDB(w http.ResponseWriter, r *http.Request) {
	var req MigrateRequest
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

	err = s.inner.migrateDB(req.TargetVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type GetDatabaseResponse struct {
	DatabaseID string                 `json:"database_id"`
	State      map[string]interface{} `json:"state"`
}

func (s *CtrlServer) GetNode(w http.ResponseWriter, r *http.Request) {
	db := s.inner.db
	stateBytes := db.Bytes()
	var stateMap map[string]interface{}
	err := json.Unmarshal(stateBytes, &stateMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := types.GetDatabaseResponse{
		DatabaseID: db.DatabaseID(),
		State:      stateMap,
	}

	respBody, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(respBody)
}

func (s *CtrlServer) GetRoutes() []api.Route {
	return []api.Route{
		api.NewBasicRoute(http.MethodGet, "/node/processes/list", s.ListProcesses),
		api.NewBasicRoute(http.MethodPost, "/node/processes/start", s.StartProcess),
		api.NewBasicRoute(http.MethodPost, "/node/processes/stop", s.StopProcess),
		api.NewBasicRoute(http.MethodGet, "/node/state", s.GetNodeState),
		api.NewBasicRoute(http.MethodPost, "/node/apps/{user_id}/install", s.InstallApp),
		api.NewBasicRoute(http.MethodPost, "/node/apps/uninstall", s.UninstallApp),
		api.NewBasicRoute(http.MethodPost, "/node/users", s.AddUser),
		api.NewBasicRoute(http.MethodPost, "/node/db/migrate", s.MigrateDB),
		api.NewBasicRoute(http.MethodGet, "/node", s.GetNode),
	}
}
