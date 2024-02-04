package types

import "github.com/eagraf/habitat-new/internal/node/habitat_db/state/schemas/node"

type GetNodeResponse struct {
	State map[string]interface{} `json:"state"`
}

type PostUserRequest struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
}

type PostAppRequest struct {
	*node.AppInstallation
}
