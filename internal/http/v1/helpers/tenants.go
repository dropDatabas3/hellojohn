package helpers

import (
	"context"
	"errors"

	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/infra/v1/tenantsql"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

// tenantRepoKey is the context key to store a per-request tenant repository.
type tenantRepoKey struct{}

// WithTenantRepo stores the per-tenant repository in the context for reuse within the same request.
func WithTenantRepo(ctx context.Context, repo core.Repository) context.Context {
	return context.WithValue(ctx, tenantRepoKey{}, repo)
}

// GetTenantRepo retrieves a previously stored per-tenant repository from the context.
func GetTenantRepo(ctx context.Context) (core.Repository, bool) {
	r, ok := ctx.Value(tenantRepoKey{}).(core.Repository)
	return r, ok
}

// normalizeTenantSlug attempts to map a generic tenant identifier (slug or UUID)
// to the canonical slug used by the FS control-plane and TenantSQL manager.
// If the identifier already is a slug, it's returned as-is. If it's a UUID,
// it searches the tenant list by ID and returns its slug.
func normalizeTenantSlug(ctx context.Context, ident string) string {
	if ident == "" {
		return ident
	}
	if cpctx.Provider == nil {
		return ident
	}
	if _, err := cpctx.Provider.GetTenantBySlug(ctx, ident); err == nil {
		// It's a valid slug already
		return ident
	}
	// Fallback: search by ID
	if tenants, err := cpctx.Provider.ListTenants(ctx); err == nil {
		for _, t := range tenants {
			if t.ID == ident {
				return t.Slug
			}
		}
	}
	return ident
}

// OpenTenantRepo opens the tenant repository (PG for Phase 4) via the TenantSQLManager.
// Other drivers should be gated with 501 at higher layers until supported.
func OpenTenantRepo(ctx context.Context, mgr *tenantsql.Manager, tenantSlug string) (core.Repository, error) {
	slug := normalizeTenantSlug(ctx, tenantSlug)
	return mgr.GetPG(ctx, slug)
}

// IsNoDBForTenant checks if error indicates missing DB configuration for tenant.
func IsNoDBForTenant(err error) bool {
	return tenantsql.IsNoDBForTenant(err)
}

// IsTenantNotFound checks if the error indicates an unknown/nonexistent tenant at runtime.
func IsTenantNotFound(err error) bool {
	return errors.Is(err, tenantsql.ErrTenantNotFound)
}

// ResolveTenantSlugAndID returns canonical slug and UUID for a given identifier (slug or UUID).
// If the tenant is unknown, it returns the original identifier for both values.
func ResolveTenantSlugAndID(ctx context.Context, ident string) (slug, id string) {
	if cpctx.Provider == nil {
		return ident, ident
	}
	if t, err := cpctx.Provider.GetTenantBySlug(ctx, ident); err == nil && t != nil {
		return t.Slug, t.ID
	}
	if tenants, err := cpctx.Provider.ListTenants(ctx); err == nil {
		for _, t := range tenants {
			if t.ID == ident {
				return t.Slug, t.ID
			}
		}
	}
	return ident, ident
}
