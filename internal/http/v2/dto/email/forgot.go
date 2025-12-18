package email

type ForgotRequest struct {
	Email string `json:"email"`
}

type ForgotResponse struct {
	OK bool `json:"ok"`
}
