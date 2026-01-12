package social

import (
	"context"

	cpctx "github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
)

// cpctxAdapter adapts cpctx.Provider to TenantProvider interface.
type cpctxAdapter struct{}

// NewTenantProviderFromCpctx creates a TenantProvider that uses the global cpctx.Provider.
// This is the production adapter for Social V2 services.
func NewTenantProviderFromCpctx() TenantProvider {
	return &cpctxAdapter{}
}

// GetTenantBySlug delegates to cpctx.Provider.
func (a *cpctxAdapter) GetTenantBySlug(ctx context.Context, slug string) (*controlplane.Tenant, error) {
	if cpctx.Provider == nil {
		return nil, ErrTenantRequired
	}
	return cpctx.Provider.GetTenantBySlug(ctx, slug)
}
