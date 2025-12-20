// Package admin contiene los services administrativos V2.
package admin

import (
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	emailv2 "github.com/dropDatabas3/hellojohn/internal/email/v2"
)

// Deps contiene las dependencias para crear los services admin.
type Deps struct {
	ControlPlane controlplane.Service
	Email        emailv2.Service
}

// Services agrupa todos los services del dominio admin.
type Services struct {
	Clients  ClientService
	Consents ConsentService
	Users    UserActionService
	Scopes   ScopeService
	RBAC     RBACService
}

// NewServices crea el agregador de services admin.
func NewServices(d Deps) Services {
	return Services{
		Clients:  NewClientService(d.ControlPlane),
		Scopes:   NewScopeService(d.ControlPlane),
		Users:    NewUserActionService(d.Email),
		Consents: NewConsentService(),
		RBAC:     NewRBACService(),
	}
}
