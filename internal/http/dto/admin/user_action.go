// Package admin contiene DTOs para endpoints administrativos.
package admin

// UserActionRequest representa la entrada para acciones admin sobre usuarios.
type UserActionRequest struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Duration string `json:"duration,omitempty"` // "24h", "2h30m"
}

// UserActionResponse es la respuesta para acciones exitosas.
type UserActionResponse struct {
	Status string `json:"status"`
}
