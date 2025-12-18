package repository

import (
	"context"
	"time"
)

// Scope representa un scope OAuth.
type Scope struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	System      bool // true para scopes built-in (openid, email, profile)
	CreatedAt   time.Time
}

// ScopeRepository define operaciones sobre OAuth scopes.
type ScopeRepository interface {
	// Create crea un nuevo scope.
	// Retorna ErrConflict si el nombre ya existe.
	Create(ctx context.Context, tenantID, name, description string) (*Scope, error)

	// GetByName busca un scope por nombre dentro de un tenant.
	GetByName(ctx context.Context, tenantID, name string) (*Scope, error)

	// List lista todos los scopes de un tenant.
	List(ctx context.Context, tenantID string) ([]Scope, error)

	// UpdateDescription actualiza solo la descripción de un scope.
	// El nombre no se puede cambiar para preservar consents existentes.
	UpdateDescription(ctx context.Context, tenantID, scopeID, description string) error

	// Delete elimina un scope.
	// Retorna error si el scope está en uso por algún client.
	Delete(ctx context.Context, tenantID, scopeID string) error

	// Upsert crea un scope si no existe, o actualiza su descripción si ya existe.
	// Retorna el scope resultante.
	Upsert(ctx context.Context, tenantID, name, description string) (*Scope, error)
}
