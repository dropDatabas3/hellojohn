// Package auth contiene los controllers de autenticación V2.
package auth

import svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/auth"

// Controllers agrupa todos los controllers del dominio auth.
type Controllers struct {
	Login *LoginController
	// Refresh *RefreshController  // futuro
	// Register *RegisterController // futuro
}

// NewControllers crea el agregador de controllers auth.
func NewControllers(s svc.Services) *Controllers {
	return &Controllers{
		Login: NewLoginController(s.Login),
	}
}
