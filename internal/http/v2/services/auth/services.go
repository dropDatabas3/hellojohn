// Package auth contiene los services de autenticación V2.
package auth

import (
	"time"

	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// Deps contiene las dependencias para crear los services auth.
type Deps struct {
	DAL        store.DataAccessLayer
	Issuer     *jwtx.Issuer
	RefreshTTL time.Duration
	ClaimsHook ClaimsHook // nil = NoOp
}

// Services agrupa todos los services del dominio auth.
type Services struct {
	Login LoginService
	// Refresh RefreshService  // futuro
	// Register RegisterService // futuro
}

// NewServices crea el agregador de services auth.
func NewServices(d Deps) Services {
	return Services{
		Login: NewLoginService(LoginDeps{
			DAL:        d.DAL,
			Issuer:     d.Issuer,
			RefreshTTL: d.RefreshTTL,
			ClaimsHook: d.ClaimsHook,
		}),
	}
}
