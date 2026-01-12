package oauth

import "time"

// ConsentAcceptRequest is the input for POST /auth/consent/accept.
type ConsentAcceptRequest struct {
	Token   string `json:"consent_token"`
	Approve bool   `json:"approve"`
}

// ConsentChallenge mimics the structure cached by Authorize handler in V1.
// Used to unmarshal the consent_token payload.
type ConsentChallenge struct {
	UserID              string    `json:"user_id"`
	ClientID            string    `json:"client_id"`
	TenantID            string    `json:"tenant_id"`
	RedirectURI         string    `json:"redirect_uri"`
	RequestedScopes     []string  `json:"requested_scopes"`
	State               string    `json:"state"`
	Nonce               string    `json:"nonce"`
	CodeChallenge       string    `json:"code_challenge"`
	CodeChallengeMethod string    `json:"code_challenge_method"`
	AMR                 []string  `json:"amr"`
	ExpiresAt           time.Time `json:"expires_at"`
}

// AuthCodeRedirect contains the result location for the client.
type AuthCodeRedirect struct {
	URL string
}
