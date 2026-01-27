// Package admin contiene DTOs para endpoints administrativos.
package admin

// ScopeRequest representa la entrada para crear/actualizar un scope.
type ScopeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	System      bool   `json:"system,omitempty"`
}

// ScopeResponse representa un scope en la respuesta.
type ScopeResponse struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	System      bool   `json:"system,omitempty"`
}

// ScopeListResponse es una lista de scopes.
type ScopeListResponse []ScopeResponse
