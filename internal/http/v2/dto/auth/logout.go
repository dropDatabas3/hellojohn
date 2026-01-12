package auth

// LogoutRequest represents the request body for POST /v2/auth/logout
type LogoutRequest struct {
	// TenantID is optional; resolved from context if omitted.
	TenantID string `json:"tenant_id,omitempty"`
	// ClientID is required and must match the refresh token's client.
	ClientID string `json:"client_id"`
	// RefreshToken is the token to revoke.
	RefreshToken string `json:"refresh_token"`
}

// LogoutAllRequest represents the request body for POST /v2/auth/logout-all
type LogoutAllRequest struct {
	// UserID is required - the user whose sessions will be revoked.
	UserID string `json:"user_id"`
	// ClientID is optional - if provided, only revoke tokens for this client.
	ClientID string `json:"client_id,omitempty"`
}
