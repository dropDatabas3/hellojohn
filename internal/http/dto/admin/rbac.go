// Package admin contiene DTOs para endpoints administrativos.
package admin

// RBACRolesUpdateRequest representa la entrada para actualizar roles de usuario.
type RBACRolesUpdateRequest struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// RBACUserRolesResponse representa los roles de un usuario.
type RBACUserRolesResponse struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// RBACPermsUpdateRequest representa la entrada para actualizar permisos de un rol.
type RBACPermsUpdateRequest struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// RBACRolePermsResponse representa los permisos de un rol.
type RBACRolePermsResponse struct {
	TenantID string   `json:"tenant_id"`
	Role     string   `json:"role"`
	Perms    []string `json:"perms"`
}
