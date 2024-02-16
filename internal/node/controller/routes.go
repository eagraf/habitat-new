package controller

import (
	"encoding/json"
	"net/http"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/gorilla/mux"
)

// InstallAppRoute calls nodeController.InstallApp()
type InstallAppRoute struct {
	nodeController NodeController
}

func NewInstallAppRoute(nodeController NodeController) *InstallAppRoute {
	return &InstallAppRoute{
		nodeController: nodeController,
	}
}

func (h *InstallAppRoute) Pattern() string {
	return "/node/users/{user_id}/apps"
}

func (h *InstallAppRoute) Method() string {
	return http.MethodPost
}

func (h *InstallAppRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method, require POST", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	userID := vars["user_id"]

	var req types.PostAppRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.nodeController.InstallApp(userID, req.AppInstallation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO validate request
	w.WriteHeader(http.StatusCreated)
}

// GetNodeRoute gets the node's database and returns its state map.
type GetNodeRoute struct {
	dbManager hdb.HDBManager
}

func NewGetNodeRoute(dbManager hdb.HDBManager) *GetNodeRoute {
	return &GetNodeRoute{
		dbManager: dbManager,
	}
}

func (h *GetNodeRoute) Pattern() string {
	return "/node"
}

func (h *GetNodeRoute) Method() string {
	return http.MethodGet
}

func (h *GetNodeRoute) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	dbClient, err := h.dbManager.GetDatabaseClientByName(constants.NodeDBDefaultName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stateBytes := dbClient.Bytes()
	var stateMap map[string]interface{}
	err = json.Unmarshal(stateBytes, &stateMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := types.GetDatabaseResponse{
		DatabaseID: dbClient.DatabaseID(),
		State:      stateMap,
	}

	respBody, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
}

// AddUserRoute calls nodeController.AddUser()
type AddUserRoute struct {
	nodeController NodeController
}

func NewAddUserRoute(nodeController NodeController) *AddUserRoute {
	return &AddUserRoute{
		nodeController: nodeController,
	}
}

func (h *AddUserRoute) Pattern() string {
	return "/node/users"
}

func (h *AddUserRoute) Method() string {
	return http.MethodPost
}

func (h *AddUserRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method, require POST", http.StatusMethodNotAllowed)
		return
	}

	var req types.PostAddUserRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.nodeController.AddUser(req.UserID, req.Username, req.Certificate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO validate request
	w.WriteHeader(http.StatusCreated)
}
