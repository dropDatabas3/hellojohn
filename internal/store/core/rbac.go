package core

import "context"

// RBACRepository es opcional; si el store lo implementa, los handlers
// poblarán roles/perms en el access token (claims_ver=1).
type RBACRepository interface {
	// Devuelve los roles asignados al usuario (p.ej. ["sys:admin","billing:manager"]).
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
	// Devuelve el conjunto de permisos efectivos del usuario
	// (derivados de sus roles), p.ej. ["users:read","users:write"].
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
}
