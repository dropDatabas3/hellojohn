package social

import "context"

// ProvidersService resuelve providers sociales (y metadatos) para el "dynamic".
type ProvidersService interface {
	List(ctx context.Context, tenantID string) ([]string, error)
}
