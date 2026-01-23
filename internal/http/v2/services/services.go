// Package services agrupa todos los services HTTP V2.
// Este es el "composition root" de services.
//
//  1. CREAR EL SUB-PAQUETE:
//     internal/http/v2/services/{dominio}/
//     - {nombre}_service.go  → implementación del service
//     - services.go          → aggregator del dominio
//
// 2. DEFINIR EL AGGREGATOR DEL DOMINIO (services/{dominio}/services.go):
//
//	type Deps struct {
//	    // inyectar dependencias necesarias
//	}
//
//	type Services struct {
//	    MiService MiServiceInterface
//	}
//
//	func NewServices(d Deps) Services {
//	    return Services{
//	        MiService: NewMiService(d.AlgunaDep),
//	    }
//	}
//
// 3. AGREGAR AL AGGREGATOR PRINCIPAL (este archivo):
//   - Importar el paquete del dominio
//   - Agregar campo al struct Services
//   - Inicializar en el constructor New()
//
// 4. USO EN app.go o main.go:
//
//	deps := services.Deps{
//	    DAL:          dal,
//	    Issuer:       issuer,
//	    JWKSCache:    jwksCache,
//	    ControlPlane: cp,
//	    Email:        emailSvc,
//	    BaseIssuer:   cfg.BaseIssuer,
//	    RefreshTTL:   cfg.RefreshTTL,
//	    HealthDeps:   healthDeps,
//	}
//
//	svcs := services.New(deps)
//	// svcs.Admin.Clients, svcs.Auth.Login, svcs.OIDC.Discovery, etc.
//
// ═══════════════════════════════════════════════════════════════════════════════
package services

import (
	"time"

	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	emailv2 "github.com/dropDatabas3/hellojohn/internal/email/v2"
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/session"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/admin"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/auth"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/email"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/health"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/oauth"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/oidc"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/security"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/session"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services/social"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// Deps contiene las dependencias base para crear los services.
// Todas las dependencias externas se inyectan aquí.
type Deps struct {
	// ─── Infraestructura ───
	DAL          store.DataAccessLayer // Acceso a datos por tenant
	Issuer       *jwtx.Issuer          // Emisor JWT (keys, TTLs)
	JWKSCache    *jwtx.JWKSCache       // Cache de JWKS público
	ControlPlane controlplane.Service  // Operaciones FS (tenants, clients, scopes)
	Email        emailv2.Service       // Servicio de emails

	// ─── Configuración ───
	BaseIssuer string        // Issuer base (ej: "https://auth.example.com")
	RefreshTTL time.Duration // TTL para refresh tokens

	// ─── Health Check ───
	HealthDeps health.Deps // Dependencias específicas para health probes
	MasterKey  string      // Master key hex para cifrado

	// ─── Social V2 ───
	SocialCache        social.CacheWriter // Cache con write capabilities para social
	SocialDebugPeek    bool               // Debug peek mode para result viewer
	SocialOIDCFactory  social.OIDCFactory // Factory para OIDC clients (Google, etc.)
	SocialStateSigner  social.StateSigner // Signer para state JWTs
	SocialLoginCodeTTL time.Duration      // TTL para login codes (default 60s)
	Social             social.Services    // Social services

	// ─── Auth Feature Flags ───
	AutoLogin      bool // Auto-login after registration
	FSAdminEnabled bool // Allow FS-admin registration

	// ─── OAuth V2 ───
	OAuthCache       oauth.CacheClient
	OAuthCookieName  string
	OAuthAllowBearer bool
}

// Services agrupa todos los sub-services por dominio.
// Cada dominio tiene su propio aggregator en su sub-paquete.
type Services struct {
	Admin    admin.Services   // Operaciones admin
	Auth     auth.Services    // Autenticación
	OIDC     oidc.Services    // OIDC
	OAuth    oauth.Services   // OAuth2 (authorize, token)
	Session  session.Services // Session management
	Email    email.Services   // Email flows
	Security security.Services
	Health   health.Services
	Social   social.Services
}

// New crea el agregador de services con todas las dependencias inyectadas.
// Este es el único lugar donde se instancian los services.
func New(d Deps) *Services {
	return &Services{
		Admin: admin.NewServices(admin.Deps{
			DAL:          d.DAL,
			ControlPlane: d.ControlPlane,
			Email:        d.Email,
			MasterKey:    d.MasterKey,
			Issuer:       d.Issuer,
			RefreshTTL:   d.RefreshTTL,
		}),
		Auth: auth.NewServices(auth.Deps{
			DAL:            d.DAL,
			Issuer:         d.Issuer,
			RefreshTTL:     d.RefreshTTL,
			ClaimsHook:     nil, // NoOp por defecto, inyectar si se necesita
			AutoLogin:      d.AutoLogin,
			FSAdminEnabled: d.FSAdminEnabled,
			Email:          d.Email,
			Social:         d.Social,
		}),
		OIDC: oidc.NewServices(oidc.Deps{
			JWKSCache:    d.JWKSCache,
			BaseIssuer:   d.BaseIssuer,
			ControlPlane: d.ControlPlane,
			Issuer:       d.Issuer,
			DAL:          d.DAL,
		}),
		Health: health.NewServices(d.HealthDeps),
		Social: d.Social,
		OAuth: oauth.NewServices(oauth.Deps{
			DAL:          d.DAL,
			Issuer:       d.Issuer,
			RefreshTTL:   d.RefreshTTL,
			Cache:        d.OAuthCache,
			ControlPlane: d.ControlPlane,
			CookieName:   d.OAuthCookieName,
			AllowBearer:  d.OAuthAllowBearer,
		}),
		Session: session.NewServices(session.Deps{
			Cache:        nil,
			LogoutConfig: dto.SessionLogoutConfig{},
			LoginConfig:  dto.LoginConfig{},
		}),
		Email: email.NewServices(email.Deps{
			Email:          d.Email,
			ControlPlane:   d.ControlPlane,
			VerifyTTL:      48 * time.Hour,
			ResetTTL:       1 * time.Hour,
			AutoLoginReset: d.AutoLogin,
			Policy:         nil, // Generar policy real si configuración lo requiere
			Issuer:         nil, // Implementar TokenIssuer adapter para soporte AutoLogin
		}),
		Security: security.NewServices(security.Deps{
			// Add security deps if any
		}),
	}
}
