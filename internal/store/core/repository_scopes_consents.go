package core

import (
	"context"
	"time"
)

// ScopesConsentsRepository define operaciones CRUD y de gobierno.
type ScopesConsentsRepository interface {
	// Scopes
	CreateScope(ctx context.Context, tenantID, name, description string) (Scope, error)
	GetScopeByName(ctx context.Context, tenantID, name string) (Scope, error)
	ListScopes(ctx context.Context, tenantID string) ([]Scope, error)
	DeleteScope(ctx context.Context, tenantID, scopeID string) error
	// Variante por ID directo
	DeleteScopeByID(ctx context.Context, scopeID string) error
	// Update solo de descripción (no renombrar para preservar consents)
	UpdateScopeDescription(ctx context.Context, tenantID, scopeID, description string) error
	// Variante por ID directo (sin tenant) para usos admin cuando ya hay aislamiento a nivel auth
	UpdateScopeDescriptionByID(ctx context.Context, scopeID, description string) error

	// Consents
	UpsertConsent(ctx context.Context, userID, clientID string, scopes []string) (UserConsent, error)
	GetConsent(ctx context.Context, userID, clientID string) (UserConsent, error)
	ListConsentsByUser(ctx context.Context, userID string, activeOnly bool) ([]UserConsent, error)
	RevokeConsent(ctx context.Context, userID, clientID string, at time.Time) error
}
