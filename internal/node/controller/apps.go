package controller

import (
	"encoding/json"
	"net/http"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/gorilla/mux"
)

// Handlers

type PostAppHandler struct {
	nodeController NodeController
}

func NewPostAppHandler(nodeController NodeController) *PostAppHandler {
	return &PostAppHandler{
		nodeController: nodeController,
	}
}

func (h *PostAppHandler) Pattern() string {
	return "/node/users/{user_id}/apps"
}

func (h *PostAppHandler) Method() string {
	return http.MethodPost
}

func (h *PostAppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

// Controller methods

func (c *BaseNodeController) InstallApp(userID string, newApp *node.AppInstallation) error {
	dbClient, err := c.databaseManager.GetDatabaseClientByName(NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = dbClient.ProposeTransitions([]hdb.Transition{
		&node.StartInstallationTransition{
			UserID:          userID,
			AppInstallation: newApp,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
