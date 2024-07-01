package types

type PDSInviteCodeResponse struct {
	Code string `json:"code"`
}

type PDSCreateAccountRequest struct {
	Email      string `json:"email"`
	Handle     string `json:"handle"`
	Password   string `json:"password"`
	InviteCode string `json:"inviteCode"`
}

type PDSCreateAccountResponse map[string]interface{}
