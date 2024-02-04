package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/internal/node/habitat_db"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state"
	"github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"
)

// Handlers

type GetNodeHandler struct {
	dbManager *habitat_db.DatabaseManager
}

func NewGetNodeHandler(dbManager *habitat_db.DatabaseManager) *GetNodeHandler {
	return &GetNodeHandler{
		dbManager: dbManager,
	}
}

func (h *GetNodeHandler) Pattern() string {
	return "/node"
}

func (h *GetNodeHandler) Method() string {
	return http.MethodGet
}

func (h *GetNodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	db, err := h.dbManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stateBytes, err := db.Bytes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var stateMap map[string]interface{}
	err = json.Unmarshal(stateBytes, &stateMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := types.GetDatabaseResponse{
		DatabaseID: db.ID,
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

type PostUserHandler struct {
	nodeController NodeController
}

func NewPostUserHandler(nodeController NodeController) *PostUserHandler {
	return &PostUserHandler{
		nodeController: nodeController,
	}
}

func (h *PostUserHandler) Pattern() string {
	return "/node/users"
}

func (h *PostUserHandler) Method() string {
	return http.MethodPost
}

func (h *PostUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method, require POST", http.StatusMethodNotAllowed)
		return
	}

	var req types.PostUserRequest
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

// Controller methods

func (c *BaseNodeController) AddUser(userID, username, certificate string) error {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return err
	}

	_, err = db.Controller.ProposeTransitions([]state.Transition{
		&node.AddUserTransition{
			UserID:      userID,
			Username:    username,
			Certificate: certificate,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *BaseNodeController) GetUserByUsername(username string) (*node.User, error) {
	db, err := c.databaseManager.GetDatabaseByName(NodeDBDefaultName)
	if err != nil {
		return nil, err
	}

	var nodeState node.NodeState
	err = json.Unmarshal(db.Controller.Bytes(), &nodeState)
	if err != nil {
		return nil, err
	}

	for _, user := range nodeState.Users {
		if user.Username == username {
			return user, err
		}
	}

	return nil, fmt.Errorf("user with username %s not found", username)
}
