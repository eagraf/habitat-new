package types

import "github.com/eagraf/habitat-new/core/state/node"

type GetNodeResponse struct {
	State map[string]interface{} `json:"state"`
}

type PostAddUserRequest struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
}

type PostAppRequest struct {
	*node.AppInstallation
}

type PostProcessRequest struct {
	*node.Process
}
