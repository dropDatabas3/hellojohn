// Package admin contiene los controllers administrativos V2.
package admin

import (
	svc "github.com/dropDatabas3/hellojohn/internal/http/services/admin"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// Controllers agrupa todos los controllers del dominio admin.
type Controllers struct {
	Auth      *AuthController
	Clients   *ClientsController
	Consents  *ConsentsController
	Users     *UsersController
	UsersCRUD *UsersCRUDController
	Scopes    *ScopesController
	Claims    *ClaimsController
	RBAC      *RBACController
	Tenants   *TenantsController
	Tokens    *TokensController
	Sessions  *SessionsController
	Keys      *KeysController
	Cluster   *ClusterController
}

// ControllerDeps contiene dependencias adicionales para controllers.
type ControllerDeps struct {
	DAL store.DataAccessLayer
}

// NewControllers crea el agregador de controllers admin.
func NewControllers(s svc.Services, deps ControllerDeps) *Controllers {
	return &Controllers{
		Auth:     NewAuthController(s.Auth),
		Clients:  NewClientsController(s.Clients),
		Consents: NewConsentsController(s.Consents),
		Users:    NewUsersController(s.Users),
		// UsersCRUD ahora recibe actionService y DAL para soportar acciones tenant-scoped
		UsersCRUD: NewUsersCRUDControllerWithActions(s.UserCRUD, s.Users, deps.DAL),
		Scopes:    NewScopesController(s.Scopes),
		Claims:    NewClaimsController(s.Claims),
		RBAC:      NewRBACController(s.RBAC),
		Tenants:   NewTenantsController(s.Tenants),
		Tokens:    NewTokensController(s.TokensAdmin),
		Sessions:  NewSessionsController(s.SessionsAdmin),
		Keys:      NewKeysController(s.Keys),
		Cluster:   NewClusterController(s.Cluster),
	}
}
