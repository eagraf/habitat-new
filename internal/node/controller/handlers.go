package controller

import (
	"encoding/json"
	"net/http"

	"github.com/eagraf/habitat-new/api/types"
	"github.com/eagraf/habitat-new/internal/node/habitat_db"
)

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
	nodeController *NodeController
}

func NewPostUserHandler(nodeController *NodeController) *PostUserHandler {
	return &PostUserHandler{
		nodeController: nodeController,
	}
}

func (h *PostUserHandler) Pattern() string {
	return "/node/user"
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

	err = h.nodeController.AddUser(req.UserID, req.Username, req.PublicKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO validate request
	w.WriteHeader(http.StatusCreated)
}
