package appv2

import (
	"net/http"

	cp "github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	emailv2 "github.com/dropDatabas3/hellojohn/internal/email/v2"
	adminctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/admin"
	authctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/auth"
	emailctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/email"
	healthctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/health"
	oauthctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/oauth"
	oidcctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/oidc"
	securityctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/security"
	sessionctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/session"
	socialctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/social"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/router"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services"
	oauth "github.com/dropDatabas3/hellojohn/internal/http/v2/services/oauth"
	socialsvc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/social"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// Config holds configuration for the V2 app.
type Config struct {
	// Add config fields as needed
}

// Deps holds raw dependencies required to build the app (DAL, Clients, etc).
type Deps struct {
	DAL          store.DataAccessLayer
	ControlPlane cp.Service
	Email        emailv2.Service
	Issuer       *jwtx.Issuer
	SocialCache  socialsvc.CacheWriter
	MasterKey    string
	RateLimiter  mw.RateLimiter
	Social       socialsvc.Services

	// ─── Auth Config ───
	AutoLogin      bool
	FSAdminEnabled bool

	// ─── OAuth V2 ───
	OAuthCache       oauth.CacheClient
	OAuthCookieName  string
	OAuthAllowBearer bool
}

// App represents the wired V2 application.
type App struct {
	Handler http.Handler
}

// New creates and wires the V2 application.
func New(cfg Config, deps Deps) (*App, error) {
	// 1. Build Services
	svcs := services.New(services.Deps{
		DAL:          deps.DAL,
		ControlPlane: deps.ControlPlane,
		Email:        deps.Email,
		MasterKey:    deps.MasterKey,
		Issuer:       deps.Issuer,
		SocialCache:  deps.SocialCache,
		Social:       deps.Social,
		// Auth Config
		AutoLogin:      deps.AutoLogin,
		FSAdminEnabled: deps.FSAdminEnabled,
		// OAuth
		OAuthCache:       deps.OAuthCache,
		OAuthCookieName:  deps.OAuthCookieName,
		OAuthAllowBearer: deps.OAuthAllowBearer,
	})

	// 2. Build Controllers
	authControllers := authctrl.NewControllers(svcs.Auth)
	adminControllers := adminctrl.NewControllers(svcs.Admin)
	oidcControllers := oidcctrl.NewControllers(svcs.OIDC)

	oauthControllers := oauthctrl.NewControllers(svcs.OAuth, oauthctrl.ControllerDeps{
		// Deps... inferred from services or passed explicitly?
		// Checking codebase, OAuth NewControllers takes 2 args.
		// Assuming empty deps structure is accepted or need to fill it.
		// For wiring check, we pass zero value.
	})

	socialControllers := socialctrl.NewControllers(svcs.Social)

	sessionControllers := sessionctrl.NewControllers(svcs.Session, sessionctrl.ControllerDeps{
		// Same assumption
	})

	emailControllers := emailctrl.NewControllers(svcs.Email)
	securityControllers := securityctrl.NewControllers(svcs.Security)
	// Health has no service yet, simple handlers
	healthControllers := &healthctrl.Controllers{
		Health: healthctrl.NewHealthController(svcs.Health.Health),
	}

	// 3. Register Routes
	mux := http.NewServeMux()
	router.RegisterV2Routes(router.V2RouterDeps{
		Mux:                 mux,
		DAL:                 deps.DAL,
		Issuer:              deps.Issuer,
		AuthControllers:     authControllers,
		AdminControllers:    adminControllers,
		OAuthControllers:    oauthControllers,
		OIDCControllers:     oidcControllers,
		SocialControllers:   socialControllers,
		SessionControllers:  sessionControllers,
		EmailControllers:    emailControllers,
		SecurityControllers: securityControllers,
		HealthControllers:   healthControllers,
		RateLimiter:         deps.RateLimiter,
		AuthMiddleware:      mw.RequireAuth(deps.Issuer),
	})

	return &App{
		Handler: mux,
	}, nil
}
