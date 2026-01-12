// Package auth contiene los services de autenticación V2.
package auth

import (
	"time"

	"github.com/dropDatabas3/hellojohn/internal/cache/v2"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// Deps contiene las dependencias para crear los services auth.
type Deps struct {
	DAL            store.DataAccessLayer
	Issuer         *jwtx.Issuer
	Cache          cache.Client
	RefreshTTL     time.Duration
	ClaimsHook     ClaimsHook     // nil = NoOp
	BlacklistPath  string         // Password blacklist path (optional)
	AutoLogin      bool           // Auto-login after registration
	FSAdminEnabled bool           // Allow FS-admin registration
	DataRoot       string         // Data root for logo file reading
	Providers      ProviderConfig // Global provider configuration
}

// Services agrupa todos los services del dominio auth.
type Services struct {
	Login           LoginService
	Refresh         RefreshService
	Logout          LogoutService
	Register        RegisterService
	Config          ConfigService
	Providers       ProvidersService
	CompleteProfile CompleteProfileService
	Profile         ProfileService
	MFATOTP         MFATOTPService
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
		Refresh: NewRefreshService(RefreshDeps{
			DAL:        d.DAL,
			Issuer:     d.Issuer,
			RefreshTTL: d.RefreshTTL,
			ClaimsHook: d.ClaimsHook,
		}),
		Logout: NewLogoutService(LogoutDeps{
			DAL: d.DAL,
		}),
		Register: NewRegisterService(RegisterDeps{
			DAL:            d.DAL,
			Issuer:         d.Issuer,
			RefreshTTL:     d.RefreshTTL,
			ClaimsHook:     d.ClaimsHook,
			BlacklistPath:  d.BlacklistPath,
			AutoLogin:      d.AutoLogin,
			FSAdminEnabled: d.FSAdminEnabled,
		}),
		Config: NewConfigService(ConfigDeps{
			DAL:      d.DAL,
			DataRoot: d.DataRoot,
		}),
		Providers: NewProvidersService(ProvidersDeps{
			DAL:       d.DAL,
			Providers: d.Providers,
		}),
		CompleteProfile: NewCompleteProfileService(CompleteProfileDeps{
			DAL: d.DAL,
		}),
		Profile: NewProfileService(ProfileDeps{
			DAL: d.DAL,
		}),
		MFATOTP: NewMFATOTPService(MFATOTPDeps{
			DAL:        d.DAL,
			Issuer:     d.Issuer,
			Cache:      d.Cache,
			RefreshTTL: d.RefreshTTL,
			ClaimsHook: d.ClaimsHook,
		}),
	}
}
