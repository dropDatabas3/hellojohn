package session

// SessionLogoutRequest contains the query parameters for POST /v2/session/logout.
type SessionLogoutRequest struct {
	ReturnTo string `json:"return_to"` // Optional: redirect URL after logout
}

// SessionLogoutConfig contains configuration for session logout.
type SessionLogoutConfig struct {
	CookieName   string          // Cookie name (default: "sid")
	CookieDomain string          // Cookie domain
	SameSite     string          // SameSite policy ("Lax", "Strict", "None")
	Secure       bool            // Secure flag for cookie
	AllowedHosts map[string]bool // Allowed hosts for return_to redirect
}
