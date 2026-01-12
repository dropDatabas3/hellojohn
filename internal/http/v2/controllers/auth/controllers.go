// Package auth contiene los controllers de autenticación V2.
package auth

import svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/auth"

// Controllers agrupa todos los controllers del dominio auth.
type Controllers struct {
	Login           *LoginController
	Refresh         *RefreshController
	Logout          *LogoutController
	Register        *RegisterController
	Config          *ConfigController
	Providers       *ProvidersController
	CompleteProfile *CompleteProfileController
	Me              *MeController
	Profile         *ProfileController
	MFATOTP         *MFATOTPController
}

// NewControllers crea el agregador de controllers auth.
func NewControllers(s svc.Services) *Controllers {
	return &Controllers{
		Login:           NewLoginController(s.Login),
		Refresh:         NewRefreshController(s.Refresh),
		Logout:          NewLogoutController(s.Logout),
		Register:        NewRegisterController(s.Register),
		Config:          NewConfigController(s.Config),
		Providers:       NewProvidersController(s.Providers),
		CompleteProfile: NewCompleteProfileController(s.CompleteProfile),
		Me:              NewMeController(),
		Profile:         NewProfileController(s.Profile),
		MFATOTP:         NewMFATOTPController(s.MFATOTP),
	}
}
