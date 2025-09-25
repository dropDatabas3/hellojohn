package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpserver "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/handlers"
	"github.com/dropDatabas3/hellojohn/internal/infra/cachefactory"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/rate"
	"github.com/dropDatabas3/hellojohn/internal/store"
	pgdriver "github.com/dropDatabas3/hellojohn/internal/store/pg"
	rdb "github.com/redis/go-redis/v9"
)

// Adapter para que rate.Limiter cumpla con http.RateLimiter
type redisLimiterAdapter struct{ inner rate.Limiter }

// Basic auth mínimo para /oauth2/introspect (endurecido via ENV)
type basicAuthCfg struct{ user, pass string }

func (a redisLimiterAdapter) Allow(ctx context.Context, key string) (struct {
	Allowed     bool
	Remaining   int64
	RetryAfter  time.Duration
	WindowTTL   time.Duration
	CurrentHits int64
}, error) {
	res, err := a.inner.Allow(ctx, key)
	if err != nil {
		return struct {
			Allowed     bool
			Remaining   int64
			RetryAfter  time.Duration
			WindowTTL   time.Duration
			CurrentHits int64
		}{}, err
	}
	return struct {
		Allowed     bool
		Remaining   int64
		RetryAfter  time.Duration
		WindowTTL   time.Duration
		CurrentHits int64
	}{
		Allowed:     res.Allowed,
		Remaining:   res.Remaining,
		RetryAfter:  res.RetryAfter,
		WindowTTL:   res.WindowTTL,
		CurrentHits: res.CurrentHits,
	}, nil
}

// allowAllClientAuth es un validador de cliente permisivo para /oauth2/introspect (solo dev/stub)
func (a basicAuthCfg) ValidateClientAuth(r *http.Request) (string, string, bool) {
	u, p, ok := r.BasicAuth()
	if !ok {
		return "", "", false
	}
	if subtle.ConstantTimeCompare([]byte(u), []byte(a.user)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(a.pass)) == 1 {
		return "", "", true
	}
	return "", "", false
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func splitCSVEnv(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
func getenvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func loadConfigFromEnv() *config.Config {
	c := &config.Config{}

	// --- Server ---
	c.Server.Addr = getenv("SERVER_ADDR", ":8080")
	c.Server.CORSAllowedOrigins = splitCSVEnv(getenv("SERVER_CORS_ALLOWED_ORIGINS", "http://localhost:3000"))

	// --- Storage ---
	c.Storage.Driver = getenv("STORAGE_DRIVER", "postgres")
	c.Storage.DSN = getenv("STORAGE_DSN", "postgres://user:password@localhost:5432/login?sslmode=disable")
	c.Storage.Postgres.MaxOpenConns = getenvInt("POSTGRES_MAX_OPEN_CONNS", 30)
	c.Storage.Postgres.MaxIdleConns = getenvInt("POSTGRES_MAX_IDLE_CONNS", 5)
	c.Storage.Postgres.ConnMaxLifetime = getenv("POSTGRES_CONN_MAX_LIFETIME", "30m")
	c.Storage.MySQL.DSN = getenv("MYSQL_DSN", "")
	c.Storage.Mongo.URI = getenv("MONGO_URI", "")
	c.Storage.Mongo.Database = getenv("MONGO_DATABASE", "")

	// --- Cache ---
	c.Cache.Kind = getenv("CACHE_KIND", "memory")
	c.Cache.Redis.Addr = getenv("REDIS_ADDR", "localhost:6379")
	c.Cache.Redis.DB = getenvInt("REDIS_DB", 0)
	c.Cache.Redis.Prefix = getenv("REDIS_PREFIX", "login:")
	c.Cache.Memory.DefaultTTL = getenv("CACHE_MEMORY_DEFAULT_TTL", "2m")

	// --- JWT ---
	c.JWT.Issuer = getenv("JWT_ISSUER", "http://localhost:8080")
	c.JWT.AccessTTL = getenv("JWT_ACCESS_TTL", "15m")
	c.JWT.RefreshTTL = getenv("JWT_REFRESH_TTL", "720h")

	// --- Register / Auth ---
	c.Register.AutoLogin = getenvBool("REGISTER_AUTO_LOGIN", true)
	c.Auth.AllowBearerSession = getenvBool("AUTH_ALLOW_BEARER_SESSION", true)
	c.Auth.Session.CookieName = getenv("AUTH_SESSION_COOKIE_NAME", "sid")
	c.Auth.Session.Domain = getenv("AUTH_SESSION_DOMAIN", "")
	c.Auth.Session.SameSite = getenv("AUTH_SESSION_SAMESITE", "Lax")
	c.Auth.Session.Secure = getenvBool("AUTH_SESSION_SECURE", false)
	c.Auth.Session.TTL = getenv("AUTH_SESSION_TTL", "12h")

	// Reset / Verify
	if d, err := time.ParseDuration(getenv("AUTH_RESET_TTL", "1h")); err == nil {
		c.Auth.Reset.TTL = d
	} else {
		c.Auth.Reset.TTL = 60 * time.Minute
	}
	c.Auth.Reset.AutoLogin = getenvBool("AUTH_RESET_AUTO_LOGIN", true)
	if d, err := time.ParseDuration(getenv("AUTH_VERIFY_TTL", "48h")); err == nil {
		c.Auth.Verify.TTL = d
	} else {
		c.Auth.Verify.TTL = 48 * time.Hour
	}

	// --- Rate ---
	c.Rate.Enabled = getenvBool("RATE_ENABLED", true)
	c.Rate.Window = getenv("RATE_WINDOW", "1m")
	c.Rate.MaxRequests = getenvInt("RATE_MAX_REQUESTS", 60)

	// --- Flags ---
	c.Flags.Migrate = getenvBool("FLAGS_MIGRATE", true)

	// --- Introspection Basic Auth (mínimo endurecido) ---
	c.Auth.IntrospectBasicUser = getenv("INTROSPECT_BASIC_USER", "")
	c.Auth.IntrospectBasicPass = getenv("INTROSPECT_BASIC_PASS", "")

	// --- SMTP ---
	c.SMTP.Host = getenv("SMTP_HOST", "smtp.gmail.com")
	c.SMTP.Port = getenvInt("SMTP_PORT", 587)
	c.SMTP.Username = getenv("SMTP_USERNAME", "")
	c.SMTP.Password = getenv("SMTP_PASSWORD", "")
	c.SMTP.From = getenv("SMTP_FROM", c.SMTP.Username)
	c.SMTP.TLS = getenv("SMTP_TLS", "starttls")
	c.SMTP.InsecureSkipVerify = getenvBool("SMTP_INSECURE_SKIP_VERIFY", false)

	// --- Email ---
	c.Email.BaseURL = getenv("EMAIL_BASE_URL", "http://localhost:8080")
	c.Email.TemplatesDir = getenv("EMAIL_TEMPLATES_DIR", "./templates")
	c.Email.DebugEchoLinks = getenvBool("EMAIL_DEBUG_LINKS", true)

	// --- Security (password policy) ---
	c.Security.PasswordPolicy.MinLength = getenvInt("SECURITY_PASSWORD_POLICY_MIN_LENGTH", 10)
	c.Security.PasswordPolicy.RequireUpper = getenvBool("SECURITY_PASSWORD_POLICY_REQUIRE_UPPER", true)
	c.Security.PasswordPolicy.RequireLower = getenvBool("SECURITY_PASSWORD_POLICY_REQUIRE_LOWER", true)
	c.Security.PasswordPolicy.RequireDigit = getenvBool("SECURITY_PASSWORD_POLICY_REQUIRE_DIGIT", true)
	c.Security.PasswordPolicy.RequireSymbol = getenvBool("SECURITY_PASSWORD_POLICY_REQUIRE_SYMBOL", false)
	c.Security.PasswordBlacklistPath = getenv("SECURITY_PASSWORD_BLACKLIST_PATH", c.Security.PasswordBlacklistPath)

	// --- Providers / Social (ENV-only mode) ---
	// Login code TTL
	if d, err := time.ParseDuration(getenv("SOCIAL_LOGIN_CODE_TTL", "60s")); err == nil {
		c.Providers.LoginCodeTTL = d
	} else {
		c.Providers.LoginCodeTTL = 60 * time.Second
	}

	// Google
	c.Providers.Google.Enabled = getenvBool("GOOGLE_ENABLED", false)
	c.Providers.Google.ClientID = getenv("GOOGLE_CLIENT_ID", c.Providers.Google.ClientID)
	c.Providers.Google.ClientSecret = getenv("GOOGLE_CLIENT_SECRET", c.Providers.Google.ClientSecret)
	c.Providers.Google.RedirectURL = getenv("GOOGLE_REDIRECT_URL", c.Providers.Google.RedirectURL)

	if scopes := splitCSVEnv(getenv("GOOGLE_SCOPES", "")); len(scopes) > 0 {
		c.Providers.Google.Scopes = scopes
	} else if len(c.Providers.Google.Scopes) == 0 {
		// default scopes if none provided
		c.Providers.Google.Scopes = []string{"openid", "email", "profile"}
	}

	if v := splitCSVEnv(getenv("GOOGLE_ALLOWED_TENANTS", "")); v != nil {
		c.Providers.Google.AllowedTenants = v
	}
	if v := splitCSVEnv(getenv("GOOGLE_ALLOWED_CLIENTS", "")); v != nil {
		c.Providers.Google.AllowedClients = v
	}

	// Derivar redirect si está habilitado Google y no se especificó
	if c.Providers.Google.Enabled && strings.TrimSpace(c.Providers.Google.RedirectURL) == "" && strings.TrimSpace(c.JWT.Issuer) != "" {
		base := strings.TrimRight(c.JWT.Issuer, "/")
		c.Providers.Google.RedirectURL = base + "/v1/auth/social/google/callback"
	}

	// Prod: nunca echo links de debug
	if strings.EqualFold(getenv("APP_ENV", "dev"), "prod") {
		c.Email.DebugEchoLinks = false
	}

	return c
}

func printConfigSummary(c *config.Config) {
	log.Printf(`CONFIG:
  server.addr=%s
  cors=%v

  storage.driver=%s
  storage.dsn=%s

  cache.kind=%s
  redis.addr=%s db=%d prefix=%s

  jwt.issuer=%s access_ttl=%s refresh_ttl=%s

  auth.session(cookie=%s, domain=%s, samesite=%s, secure=%t, ttl=%s)
  auth.reset(ttl=%s, autologin=%t) verify(ttl=%s)

  rate(enabled=%t, window=%s, max=%d)

  smtp(host=%s, port=%d, user=%s, from=%s, tls=%s, insecure=%t)

  email(base_url=%s, templates=%s, debug_echo_links=%t)

  providers(login_code_ttl=%s)   // NUEVO

  pwd_policy(min=%d, upper=%t, lower=%t, digit=%t, symbol=%t)
	password_blacklist_path=%s
`,
		c.Server.Addr, c.Server.CORSAllowedOrigins,
		c.Storage.Driver, c.Storage.DSN,
		c.Cache.Kind, c.Cache.Redis.Addr, c.Cache.Redis.DB, c.Cache.Redis.Prefix,
		c.JWT.Issuer, c.JWT.AccessTTL, c.JWT.RefreshTTL,
		c.Auth.Session.CookieName, c.Auth.Session.Domain, c.Auth.Session.SameSite, c.Auth.Session.Secure, c.Auth.Session.TTL,
		c.Auth.Reset.TTL, c.Auth.Reset.AutoLogin, c.Auth.Verify.TTL,
		c.Rate.Enabled, c.Rate.Window, c.Rate.MaxRequests,
		c.SMTP.Host, c.SMTP.Port, c.SMTP.Username, c.SMTP.From, c.SMTP.TLS, c.SMTP.InsecureSkipVerify,
		c.Email.BaseURL, c.Email.TemplatesDir, c.Email.DebugEchoLinks,
		c.Providers.LoginCodeTTL,
		c.Security.PasswordPolicy.MinLength, c.Security.PasswordPolicy.RequireUpper, c.Security.PasswordPolicy.RequireLower, c.Security.PasswordPolicy.RequireDigit, c.Security.PasswordPolicy.RequireSymbol,
		c.Security.PasswordBlacklistPath,
	)
}

func main() {
	var (
		flagConfigPath = flag.String("config", "", "ruta a config.yaml (fallback: $CONFIG_PATH o configs/config.yaml)")
		flagEnvOnly    = flag.Bool("env", false, "usar SOLO env (y .env si se pasa -env-file)")
		flagEnvFile    = flag.String("env-file", ".env", "ruta a .env (si existe, se carga)")
		flagPrint      = flag.Bool("print-config", false, "imprime config efectiva y termina")
	)
	flag.Parse()

	if *flagEnvFile != "" && (fileExists(*flagEnvFile) || *flagEnvOnly) {
		if err := godotenv.Load(*flagEnvFile); err == nil {
			log.Printf("dotenv: cargado %s", *flagEnvFile)
		}
	}

	var cfg *config.Config
	var err error
	if *flagEnvOnly {
		cfg = loadConfigFromEnv()
	} else {
		cfgPath := *flagConfigPath
		if cfgPath == "" {
			cfgPath = os.Getenv("CONFIG_PATH")
		}
		if cfgPath == "" {
			if fileExists("configs/config.yaml") {
				cfgPath = "configs/config.yaml"
			} else {
				cfgPath = "configs/config.example.yaml"
			}
		}
		cfg, err = config.Load(cfgPath)
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		type envOverrider interface{ ApplyEnvOverrides() }
		if o, ok := any(cfg).(envOverrider); ok {
			o.ApplyEnvOverrides()
		}
	}
	if *flagPrint {
		printConfigSummary(cfg)
		return
	}

	// D) Hard check de SIGNING_MASTER_KEY (requerimos >=32 bytes para AES-256-GCM de secretos MFA y claves privadas opcionales)
	if k := strings.TrimSpace(os.Getenv("SIGNING_MASTER_KEY")); len(k) < 32 {
		log.Fatal("SIGNING_MASTER_KEY faltante o muy corta: se requieren >=32 bytes")
	}

	ctx := context.Background()

	// ───── Store compuesto (repo principal + scopes/consents opcional) ─────
	stores, err := store.OpenStores(ctx, store.Config{
		Driver: cfg.Storage.Driver,
		DSN:    cfg.Storage.DSN,
		Postgres: struct {
			MaxOpenConns, MaxIdleConns int
			ConnMaxLifetime            string
		}{
			MaxOpenConns:    cfg.Storage.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Storage.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Storage.Postgres.ConnMaxLifetime,
		},
		MySQL: struct{ DSN string }{DSN: cfg.Storage.MySQL.DSN},
		Mongo: struct{ URI, Database string }{URI: cfg.Storage.Mongo.URI, Database: cfg.Storage.Mongo.Database},
	})
	if err != nil {
		log.Fatalf("store open: %v", err)
	}
	// Para compatibilidad, usamos repo := stores.Repository
	repo := stores.Repository
	// Aseguramos cierre ordenado del pool extra (si existe)
	defer func() {
		// Si más adelante Container se crea antes, también se podría defer container.Close()
		if stores.Close != nil {
			_ = stores.Close()
		}
	}()

	// Migraciones
	if cfg.Flags.Migrate {
		if pg, ok := repo.(interface {
			RunMigrations(context.Context, string) error
		}); ok {
			if err := pg.RunMigrations(ctx, "migrations/postgres"); err != nil {
				log.Fatalf("migrations: %v", err)
			}
		} else if _, ok := repo.(*pgdriver.Store); ok {
			if err := repo.(*pgdriver.Store).RunMigrations(ctx, "migrations/postgres"); err != nil {
				log.Fatalf("migrations: %v", err)
			}
		}
	}

	// JWT / JWKS (Keystore persistente con bootstrap)
	pgRepo, ok := repo.(*pgdriver.Store)
	if !ok {
		log.Fatalf("signing keys: Postgres store requerido")
	}
	ks := jwtx.NewPersistentKeystore(ctx, pgRepo)
	if err := ks.EnsureBootstrap(); err != nil {
		log.Fatalf("bootstrap signing key: %v", err)
	}

	iss := cfg.JWT.Issuer
	if iss == "" {
		iss = "http://localhost:8080"
	}
	issuer := jwtx.NewIssuer(iss, ks)
	if cfg.JWT.AccessTTL != "" {
		if d, err := time.ParseDuration(cfg.JWT.AccessTTL); err == nil {
			issuer.AccessTTL = d
		}
	}
	refreshTTL := 30 * 24 * time.Hour
	if cfg.JWT.RefreshTTL != "" {
		if d, err := time.ParseDuration(cfg.JWT.RefreshTTL); err == nil {
			refreshTTL = d
		}
	}

	// Cache genérica
	cc, err := cachefactory.Open(cachefactory.Config{
		Kind: cfg.Cache.Kind,
		Redis: struct {
			Addr   string
			DB     int
			Prefix string
		}{
			Addr:   cfg.Cache.Redis.Addr,
			DB:     cfg.Cache.Redis.DB,
			Prefix: cfg.Cache.Redis.Prefix,
		},
		Memory: struct{ DefaultTTL string }{
			DefaultTTL: cfg.Cache.Memory.DefaultTTL,
		},
	})
	if err != nil {
		log.Fatalf("cache: %v", err)
	}

	container := app.Container{
		Store:          repo, // repo principal
		Issuer:         issuer,
		Cache:          cc,
		Stores:         stores,                // wrapper (incluye Close opcional)
		ScopesConsents: stores.ScopesConsents, // puede ser nil si driver != postgres
	}
	// Si preferís, podés delegar el cierre al contenedor:
	// defer container.Close()

	sessionTTL, _ := time.ParseDuration(cfg.Auth.Session.TTL)
	if sessionTTL == 0 {
		sessionTTL = 12 * time.Hour
	}

	jwksHandler := handlers.NewJWKSHandler(&container)
	authLoginHandler := handlers.NewAuthLoginHandler(&container, cfg, refreshTTL)
	authRegisterHandler := handlers.NewAuthRegisterHandler(&container, cfg.Register.AutoLogin, refreshTTL, cfg.Security.PasswordBlacklistPath)
	authRefreshHandler := handlers.NewAuthRefreshHandler(&container, refreshTTL)
	authLogoutHandler := handlers.NewAuthLogoutHandler(&container)
	meHandler := handlers.NewMeHandler(&container)

	authLogoutAllHandler := handlers.NewAuthLogoutAllHandler(&container)

	// Protegemos todas las rutas /v1/admin/* con RequireAuth + RequireSysAdmin

	adminScopes := httpserver.Chain(
		handlers.NewAdminScopesHandler(&container),
		httpserver.RequireAuth(container.Issuer),
		httpserver.RequireSysAdmin(container.Issuer))
	adminConsents := httpserver.Chain(
		handlers.NewAdminConsentsHandler(&container),
		httpserver.RequireAuth(container.Issuer),
		httpserver.RequireSysAdmin(container.Issuer))
	adminClients := httpserver.Chain(
		handlers.NewAdminClientsHandler(&container),
		httpserver.RequireAuth(container.Issuer),
		httpserver.RequireSysAdmin(container.Issuer))
	// ─── RBAC Admin ───
	adminRBACUsers := httpserver.Chain(
		handlers.AdminRBACUsersRolesHandler(&container),
		httpserver.RequireAuth(container.Issuer),
		httpserver.RequireSysAdmin(container.Issuer),
	)
	adminRBACRoles := httpserver.Chain(
		handlers.AdminRBACRolePermsHandler(&container),
		httpserver.RequireAuth(container.Issuer),
		httpserver.RequireSysAdmin(container.Issuer),
	)

	// Introspect endurecido con basic auth (si user/pass vacíos ⇒ siempre 401)
	introspectAuth := basicAuthCfg{user: strings.TrimSpace(cfg.Auth.IntrospectBasicUser), pass: strings.TrimSpace(cfg.Auth.IntrospectBasicPass)}
	oauthIntrospectHandler := handlers.NewOAuthIntrospectHandler(&container, introspectAuth)

	var limiter httpserver.RateLimiter
	var multiLimiter *rate.LimiterPoolAdapter // Nuevo: para rate limits específicos
	var redisPing func(context.Context) error
	if cfg.Rate.Enabled && strings.EqualFold(cfg.Cache.Kind, "redis") {
		rc := rdb.NewClient(&rdb.Options{
			Addr: cfg.Cache.Redis.Addr,
			DB:   cfg.Cache.Redis.DB,
		})

		// MultiLimiter para endpoints específicos
		multiLimiter = rate.NewLimiterPoolAdapter(rc, cfg.Cache.Redis.Prefix+"rl:")

		// Mantener limiter global para middleware existente (backward compatibility)
		if win, err := time.ParseDuration(cfg.Rate.Window); err == nil {
			rl := rate.NewRedisLimiter(rc, cfg.Cache.Redis.Prefix+"rl:", cfg.Rate.MaxRequests, win)
			limiter = redisLimiterAdapter{inner: rl}
		}
		redisPing = func(ctx context.Context) error { return rc.Ping(ctx).Err() }
	}

	// Añadir multiLimiter al container para que los handlers lo puedan usar
	container.MultiLimiter = multiLimiter

	readyzHandler := handlers.NewReadyzHandler(&container, redisPing)

	oidcDiscoveryHandler := handlers.NewOIDCDiscoveryHandler(&container)
	oauthAuthorizeHandler := handlers.NewOAuthAuthorizeHandler(&container, cfg.Auth.Session.CookieName, cfg.Auth.AllowBearerSession)
	oauthTokenHandler := handlers.NewOAuthTokenHandler(&container, refreshTTL)
	userInfoHandler := handlers.NewUserInfoHandler(&container)
	consentAcceptHandler := handlers.NewConsentAcceptHandler(&container)

	oauthRevokeHandler := handlers.NewOAuthRevokeHandler(&container)
	sessionLoginHandler := handlers.NewSessionLoginHandler(
		&container,
		cfg.Auth.Session.CookieName,
		cfg.Auth.Session.Domain,
		cfg.Auth.Session.SameSite,
		cfg.Auth.Session.Secure,
		sessionTTL,
	)
	sessionLogoutHandler := handlers.NewSessionLogoutHandler(
		&container,
		cfg.Auth.Session.CookieName,
		cfg.Auth.Session.Domain,
		cfg.Auth.Session.SameSite,
		cfg.Auth.Session.Secure,
	)

	// Email Flows
	verifyEmailStartHandler, verifyEmailConfirmHandler, forgotHandler, resetHandler, emailCleanup, err :=
		handlers.BuildEmailFlowHandlers(ctx, cfg, &container, refreshTTL)
	if err != nil {
		log.Fatalf("email flows: %v", err)
	}
	defer emailCleanup()

	// ───────── Social: Google ─────────
	googleStart, googleCallback, googleCleanup, gerr :=
		handlers.BuildGoogleSocialHandlers(ctx, cfg, &container, refreshTTL)
	if gerr != nil {
		log.Fatalf("google social: %v", gerr)
	}
	if googleCleanup != nil {
		defer googleCleanup()
	}

	// MFA TOTP handlers (si store soporta) – se registran siempre; retornarán 501 si no hay soporte
	mfa := handlers.NewMFAHandler(&container, cfg, refreshTTL)
	mfaEnrollHandler := mfa.HTTPEnroll()
	mfaVerifyHandler := mfa.HTTPVerify()
	mfaChallengeHandler := mfa.HTTPChallenge()
	mfaDisableHandler := mfa.HTTPDisable()
	mfaRecoveryRotateHandler := mfa.HTTPRecoveryRotate()

	// Social exchange (intercambio de código efímero -> tokens)
	socialExchangeHandler := handlers.NewSocialExchangeHandler(&container)

	// Mux base ampliado (incluye sprint 5 + MFA + social exchange)
	mux := httpserver.NewMux(
		jwksHandler,
		authLoginHandler,
		authRegisterHandler,
		authRefreshHandler,
		authLogoutHandler,
		meHandler,
		readyzHandler,
		oidcDiscoveryHandler,
		oauthAuthorizeHandler,
		oauthTokenHandler,
		userInfoHandler,
		// Adicionales (orden debe coincidir con firma NewMux: oauthRevoke, sessionLogin, sessionLogout, consentAccept)
		oauthRevokeHandler,
		sessionLoginHandler,
		sessionLogoutHandler,
		consentAcceptHandler,
		// Email flows
		verifyEmailStartHandler,
		verifyEmailConfirmHandler,
		forgotHandler,
		resetHandler,
		// sprint 5
		oauthIntrospectHandler,
		authLogoutAllHandler,
		// mfa
		mfaEnrollHandler,
		mfaVerifyHandler,
		mfaChallengeHandler,
		mfaDisableHandler,
		mfaRecoveryRotateHandler,
		// social exchange
		socialExchangeHandler,

		// admin
		adminScopes,
		adminConsents,
		adminClients,
		adminRBACUsers,
		adminRBACRoles,
	)

	// Rutas Google (solo si está habilitado)
	if googleStart != nil {
		mux.Handle("/v1/auth/social/google/start", googleStart)
	}
	if googleCallback != nil {
		mux.Handle("/v1/auth/social/google/callback", googleCallback)
	}

	// Discovery de providers (siempre expuesto, sólo devuelve estado/URLs)
	providersHandler := handlers.NewProvidersHandler(&container, cfg)
	mux.Handle("/v1/auth/providers", providersHandler)

	// social/result: montarlo solo si algún provider social lo usa (por ahora: Google)
	if cfg.Providers.Google.Enabled {
		socialResultHandler := handlers.NewSocialResultHandler(&container)
		mux.Handle("/v1/auth/social/result", socialResultHandler)
	}

	handler := httpserver.WithLogging(
		httpserver.WithRecover(
			httpserver.WithRequestID(
				httpserver.WithRateLimit(
					httpserver.WithSecurityHeaders(
						httpserver.WithCORS(mux, cfg.Server.CORSAllowedOrigins),
					),
					limiter,
				),
			),
		),
	)

	mode := "yaml"
	if flag.Lookup("env").Value.String() == "true" {
		mode = "env"
	}
	log.Printf("service up. mode=%s addr=%s base=%s debug_links=%t time=%s",
		mode, cfg.Server.Addr, cfg.Email.BaseURL, cfg.Email.DebugEchoLinks, time.Now().Format(time.RFC3339))

	if err := httpserver.Start(cfg.Server.Addr, handler); err != nil {
		log.Fatalf("http: %v", err)
	}
}
