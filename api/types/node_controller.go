package types

type GetNodeResponse struct {
	State map[string]interface{} `json:"state"`
}

type PostUserRequest struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Certificate string `json:"certificate"`
}