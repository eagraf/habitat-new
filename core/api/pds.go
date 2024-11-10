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

type PDSCreateSessionRequest struct {
	Identifier string `json:"identifier"` // email or handle
	Password   string `json:"password"`
}

type PDSCreateSessionResponse map[string]interface{}

type PDSGetRecordRequest struct {
	Repo       string `json:"repo"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
}

type PDSGetRecordResponse struct {
	URI   string                 `json:"uri"`
	CID   string                 `json:"cid"`
	Value map[string]interface{} `json:"value"`
}
