package jwt

// AdminAccessClaims son los claims del access token de admin
type AdminAccessClaims struct {
	AdminID   string   `json:"sub"`
	Email     string   `json:"email"`
	AdminType string   `json:"admin_type"` // "global" | "tenant"
	Tenants   []string `json:"tenants,omitempty"`
}

// AdminRefreshClaims son los claims del refresh token de admin
type AdminRefreshClaims struct {
	AdminID string `json:"sub"`
	Type    string `json:"type"` // "admin_refresh"
}
