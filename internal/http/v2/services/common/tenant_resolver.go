package common

import "context"

// TenantResolver resuelve un tenant (p.ej. por client_id u otros hints).
type TenantResolver interface {
	ResolveTenantID(ctx context.Context, clientID string) (string, error)
}
