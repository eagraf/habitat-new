package types

import "github.com/eagraf/habitat-new/core/state/node"

type MigrateRequest struct {
	TargetVersion string `json:"target_version"`
}

type GetNodeResponse struct {
	State map[string]interface{} `json:"state"`
}

type PostAddUserRequest struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
}

type PostAppRequest struct {
	AppInstallation   *node.AppInstallation    `json:"app_installation"`
	ReverseProxyRules []*node.ReverseProxyRule `json:"reverse_proxy_rules"`
}

type PostProcessRequest struct {
	AppInstallationID string `json:"app_id"`
}
