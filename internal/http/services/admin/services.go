// Package admin contiene los services administrativos V2.
package admin

import (
	"time"

	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	emailv2 "github.com/dropDatabas3/hellojohn/internal/email"
	"github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// Deps contiene las dependencias para crear los services admin.
type Deps struct {
	DAL          store.DataAccessLayer
	ControlPlane controlplane.Service
	Email        emailv2.Service
	MasterKey    string
	Issuer       *jwt.Issuer
	RefreshTTL   time.Duration // TTL para admin refresh tokens
}

// Services agrupa todos los services del dominio admin.
type Services struct {
	Auth          AuthService
	Clients       ClientService
	Consents      ConsentService
	Users         UserActionService
	UserCRUD      UserCRUDService
	Scopes        ScopeService
	Claims        ClaimsService
	RBAC          RBACService
	Tenants       TenantsService
	TokensAdmin   TokensAdminService
	SessionsAdmin *SessionsService
	Keys          KeysService
	Cluster       ClusterService
}

// NewServices crea el agregador de services admin.
func NewServices(d Deps) Services {
	return Services{
		Auth: NewAuthService(AuthServiceDeps{
			ControlPlane: d.ControlPlane,
			Issuer:       d.Issuer,
			RefreshTTL:   d.RefreshTTL,
		}),
		Clients: NewClientService(d.ControlPlane),
		Scopes:  NewScopeService(d.ControlPlane),
		Claims:  NewClaimsService(d.ControlPlane),
		Users:   NewUserActionService(d.Email),
		UserCRUD: NewUserCRUDService(UserCRUDDeps{
			DAL: d.DAL,
		}),
		Consents:      NewConsentService(),
		RBAC:          NewRBACService(),
		Tenants:       NewTenantsService(d.DAL, d.MasterKey, d.Issuer, d.Email),
		TokensAdmin:   NewTokensAdminService(TokensAdminDeps{DAL: d.DAL}),
		SessionsAdmin: NewSessionsService(d.DAL),
		Keys:          NewKeysService(d.DAL),
		Cluster:       NewClusterService(ClusterDeps{DAL: d.DAL}),
	}
}
