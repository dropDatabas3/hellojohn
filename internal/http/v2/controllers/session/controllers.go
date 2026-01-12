// Package session contains controllers for session-related endpoints.
package session

import (
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/session"
	svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/session"
)

// ControllerDeps contains additional dependencies for controllers.
type ControllerDeps struct {
	LogoutConfig dto.SessionLogoutConfig
	LoginConfig  dto.LoginConfig
}

// Controllers agrupa todos los controllers del dominio session.
type Controllers struct {
	Logout *SessionLogoutController
	Login  *LoginController
}

// NewControllers creates the session controllers aggregator.
func NewControllers(s svc.Services, deps ControllerDeps) *Controllers {
	return &Controllers{
		Logout: NewSessionLogoutController(s.Logout, deps.LogoutConfig),
		Login:  NewLoginController(s.Login, deps.LoginConfig),
	}
}
