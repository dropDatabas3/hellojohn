package session

type LogoutRequest struct {
	ReturnTo string `json:"return_to"`
}

type LogoutResponse struct {
	OK bool `json:"ok"`
}
