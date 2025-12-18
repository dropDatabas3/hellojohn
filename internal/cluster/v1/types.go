// Package clusterv1 contiene tipos y FSM acoplados a controlplane/v1.
// Este paquete existe para mantener compatibilidad con código legacy.
// El código nuevo debe usar internal/store/v2/cluster.
package clusterv1

import (
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
)

// MutationType define el catálogo de operaciones replicadas.
type MutationType string

const (
	// MutationUpsertClient crea/actualiza un cliente OIDC dentro de un tenant.
	MutationUpsertClient MutationType = "upsert_client"
	// Clients
	MutationDeleteClient MutationType = "delete_client"

	// Tenants
	MutationUpsertTenant         MutationType = "upsert_tenant"
	MutationUpdateTenantSettings MutationType = "update_tenant_settings"
	MutationDeleteTenant         MutationType = "delete_tenant"

	// Scopes
	MutationUpsertScope MutationType = "upsert_scope"
	MutationDeleteScope MutationType = "delete_scope"

	// Key rotation (Paso 6 prep)
	MutationRotateTenantKey MutationType = "rotate_tenant_key"
)

// Mutation representa una operación a replicar por Raft.
// PayloadJSON contiene un JSON específico por tipo de mutación.
type Mutation struct {
	Type       MutationType `json:"type"`
	TenantSlug string       `json:"tenantSlug"`
	TsUnix     int64        `json:"tsUnix"`
	Payload    []byte       `json:"payload"` // JSON crudo del DTO
}

// UpsertClientDTO es el payload para MutationUpsertClient.
// Se mapea directamente a controlplane.ClientInput.
type UpsertClientDTO struct {
	Name           string        `json:"name"`
	ClientID       string        `json:"clientId"`
	Type           cp.ClientType `json:"type"`
	RedirectURIs   []string      `json:"redirectUris"`
	AllowedOrigins []string      `json:"allowedOrigins,omitempty"`
	Providers      []string      `json:"providers,omitempty"`
	Scopes         []string      `json:"scopes,omitempty"`
	Secret         string        `json:"secret,omitempty"`
	// Email verification & password reset
	RequireEmailVerification bool   `json:"requireEmailVerification,omitempty"`
	ResetPasswordURL         string `json:"resetPasswordUrl,omitempty"`
	VerifyEmailURL           string `json:"verifyEmailUrl,omitempty"`
}

// DeleteClientDTO payload para eliminar un cliente
type DeleteClientDTO struct {
	ClientID string `json:"clientId"`
}

// UpsertTenantDTO payload para crear/actualizar un tenant completo
type UpsertTenantDTO struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name"`
	Slug     string            `json:"slug"`
	Settings cp.TenantSettings `json:"settings"`
}

// UpdateTenantSettingsDTO payload para actualizar settings
type UpdateTenantSettingsDTO struct {
	Settings cp.TenantSettings `json:"settings"`
}

// DeleteTenantDTO payload para borrar un tenant
type DeleteTenantDTO struct {
	// empty; slug va en Mutation.TenantSlug
}

// UpsertScopeDTO payload para crear/actualizar scope
type UpsertScopeDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	System      bool   `json:"system,omitempty"`
}

// DeleteScopeDTO payload para borrar scope
type DeleteScopeDTO struct {
	Name string `json:"name"`
}

// RotateTenantKeyDTO payload (placeholder)
type RotateTenantKeyDTO struct {
	// Pre-serialized JSON contents for the files to write under keys/{tenant}/
	// If RetiringJSON is empty, followers should remove retiring.json if it exists.
	ActiveJSON   string `json:"activeJson"`
	RetiringJSON string `json:"retiringJson,omitempty"`
	GraceSeconds int64  `json:"graceSeconds,omitempty"`
}
