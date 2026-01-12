// Package session contiene los services del dominio session.
package session

import (
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/session"
)

// Deps contiene las dependencias para crear los services session.
type Deps struct {
	Cache        Cache
	LogoutConfig dto.SessionLogoutConfig
	LoginConfig  dto.LoginConfig
}

// Services agrupa todos los services del dominio session.
type Services struct {
	Logout SessionLogoutService
	Login  LoginService
}

// NewServices crea el agregador de services session.
func NewServices(d Deps) Services {
	return Services{
		Logout: NewSessionLogoutService(SessionLogoutDeps{
			Cache:  d.Cache,
			Config: d.LogoutConfig,
		}),
		Login: NewLoginService(LoginDeps{
			Cache:  d.Cache,
			Config: d.LoginConfig,
		}),
	}
}
