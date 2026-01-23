// Package admin contiene los controllers administrativos V2.
package admin

import svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/admin"

// Controllers agrupa todos los controllers del dominio admin.
type Controllers struct {
	Auth      *AuthController
	Clients   *ClientsController
	Consents  *ConsentsController
	Users     *UsersController
	UsersCRUD *UsersCRUDController
	Scopes    *ScopesController
	RBAC      *RBACController
	Tenants   *TenantsController
}

// NewControllers crea el agregador de controllers admin.
func NewControllers(s svc.Services) *Controllers {
	return &Controllers{
		Auth:      NewAuthController(s.Auth),
		Clients:   NewClientsController(s.Clients),
		Consents:  NewConsentsController(s.Consents),
		Users:     NewUsersController(s.Users),
		UsersCRUD: NewUsersCRUDController(s.UserCRUD),
		Scopes:    NewScopesController(s.Scopes),
		RBAC:      NewRBACController(s.RBAC),
		Tenants:   NewTenantsController(s.Tenants),
	}
}
