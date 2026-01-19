package repository

import "context"

// RBACRepository define operaciones sobre roles y permisos.
// Esta interfaz es opcional: no todos los drivers la implementan.
type RBACRepository interface {
	// GetUserRoles retorna los roles asignados a un usuario.
	// Ejemplo: ["sys:admin", "billing:manager"]
	GetUserRoles(ctx context.Context, userID string) ([]string, error)

	// GetUserPermissions retorna los permisos efectivos de un usuario.
	// Derivados de sus roles. Ejemplo: ["users:read", "users:write"]
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)

	// AssignRole asigna un rol a un usuario.
	AssignRole(ctx context.Context, tenantID, userID, role string) error

	// RemoveRole quita un rol de un usuario.
	RemoveRole(ctx context.Context, tenantID, userID, role string) error

	// ─── Role Permissions (opcional) ───

	// GetRolePermissions retorna los permisos de un rol.
	GetRolePermissions(ctx context.Context, tenantID, role string) ([]string, error)

	// AddPermissionToRole añade un permiso a un rol.
	AddPermissionToRole(ctx context.Context, tenantID, role, permission string) error

	// RemovePermissionFromRole quita un permiso de un rol.
	RemovePermissionFromRole(ctx context.Context, tenantID, role, permission string) error
}
