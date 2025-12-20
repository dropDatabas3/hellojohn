// Package admin contiene los controllers administrativos V2.
package admin

import svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/admin"

// Controllers agrupa todos los controllers del dominio admin.
type Controllers struct {
	Clients  *ClientsController
	Consents *ConsentsController
	Users    *UsersController
	Scopes   *ScopesController
	RBAC     *RBACController
}

// NewControllers crea el agregador de controllers admin.
func NewControllers(s svc.Services) *Controllers {
	return &Controllers{
		Clients:  NewClientsController(s.Clients),
		Consents: NewConsentsController(s.Consents),
		Users:    NewUsersController(s.Users),
		Scopes:   NewScopesController(s.Scopes),
		RBAC:     NewRBACController(s.RBAC),
	}
}
