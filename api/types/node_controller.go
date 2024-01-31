package types

type GetNodeResponse struct {
	State map[string]interface{} `json:"state"`
}

type PostUserRequest struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
}

type PostAppRequest struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Driver          string `json:"driver"`
	RegistryURLBase string `json:"registry_url_base"`
	RegistryAppID   string `json:"registry_app_id"`
	RegistryTag     string `json:"registry_tag"`
}
