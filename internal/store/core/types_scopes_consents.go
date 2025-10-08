package core

import "time"

// Scope representa un scope definido a nivel tenant.
// Ej: "openid", "email", "profile", "offline_access".
type Scope struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserConsent modela el consentimiento (usuario ↔ cliente) con el set de scopes concedidos.
type UserConsent struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	ClientID      string     `json:"client_id"`
	GrantedScopes []string   `json:"granted_scopes"`
	GrantedAt     time.Time  `json:"granted_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	// Schema Cut: Tenant+Client directo (sin FK)
	TenantID string `db:"tenant_id" json:"tenantId,omitempty"`
}
