package admin

import (
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
)

// TenantResponse represents a tenant in API responses.
type TenantResponse struct {
	ID          string                     `json:"id"`
	Slug        string                     `json:"slug"`
	Name        string                     `json:"name"`
	DisplayName string                     `json:"display_name"`
	Language    string                     `json:"language"`
	Settings    *repository.TenantSettings `json:"settings,omitempty"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}

// CreateTenantRequest represents the payload to create a new tenant.
type CreateTenantRequest struct {
	Slug        string                     `json:"slug"`
	Name        string                     `json:"name"`
	DisplayName string                     `json:"display_name"`
	Language    string                     `json:"language"`
	Settings    *repository.TenantSettings `json:"settings"`
}

// UpdateTenantRequest represents the payload to update a tenant.
type UpdateTenantRequest struct {
	Name        *string                    `json:"name"`
	DisplayName *string                    `json:"display_name"`
	Language    *string                    `json:"language"`
	Settings    *repository.TenantSettings `json:"settings"`
}
