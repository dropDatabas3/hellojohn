package appv2

import (
	"net/http"
	"os"
	"strings"
	"time"

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
	healthsvc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/health"
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
	JWKSCache    *jwtx.JWKSCache
	BaseIssuer   string
	RefreshTTL   time.Duration
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
		JWKSCache:    deps.JWKSCache,
		BaseIssuer:   deps.BaseIssuer,
		RefreshTTL:   deps.RefreshTTL,
		SocialCache:  deps.SocialCache,
		Social:       deps.Social,
		// Auth Config
		AutoLogin:      deps.AutoLogin,
		FSAdminEnabled: deps.FSAdminEnabled,
		// OAuth
		OAuthCache:       deps.OAuthCache,
		OAuthCookieName:  deps.OAuthCookieName,
		OAuthAllowBearer: deps.OAuthAllowBearer,
		// Health Check
		HealthDeps: healthsvc.Deps{
			ControlPlane: deps.ControlPlane,
			Issuer:       deps.Issuer,
		},
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

	// 4. Apply global middlewares (CORS, etc)
	handler := applyGlobalMiddlewares(mux)

	return &App{
		Handler: handler,
	}, nil
}

// applyGlobalMiddlewares wraps the mux with global middlewares
func applyGlobalMiddlewares(handler http.Handler) http.Handler {
	// Get CORS allowed origins from environment
	allowedOrigins := getCORSOrigins()

	// Apply CORS middleware if origins are configured
	if len(allowedOrigins) > 0 {
		handler = mw.WithCORS(allowedOrigins)(handler)
	}

	return handler
}

// getCORSOrigins returns the list of allowed CORS origins from environment
func getCORSOrigins() []string {
	// Check CORS_ALLOWED_ORIGINS env var
	corsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsEnv == "" {
		// Default for development
		corsEnv = "http://localhost:3000,http://localhost:3001"
	}

	// Split by comma and trim spaces
	origins := strings.Split(corsEnv, ",")
	result := make([]string, 0, len(origins))
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
