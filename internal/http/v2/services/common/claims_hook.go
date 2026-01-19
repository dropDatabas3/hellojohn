package common

import "context"

// ClaimsHook permite extender claims de forma opcional por tenant.
type ClaimsHook interface {
	Enrich(ctx context.Context, claims map[string]any) (map[string]any, error)
}
