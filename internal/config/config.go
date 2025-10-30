package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Bloque app (opcional en YAML). Si no está, queda vacío.
	App struct {
		// dev | staging | prod
		Env string `yaml:"app_env"`
	} `yaml:"app"`

	Server struct {
		Addr               string   `yaml:"addr"`
		CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	} `yaml:"server"`

	Storage struct {
		Driver   string `yaml:"driver"`
		DSN      string `yaml:"dsn"`
		Postgres struct {
			MaxOpenConns    int    `yaml:"max_open_conns"`
			MaxIdleConns    int    `yaml:"max_idle_conns"`
			ConnMaxLifetime string `yaml:"conn_max_lifetime"`
		} `yaml:"postgres"`
		MySQL struct {
			DSN string `yaml:"dsn"`
		} `yaml:"mysql"`
		Mongo struct {
			URI      string `yaml:"uri"`
			Database string `yaml:"database"`
		} `yaml:"mongo"`
	} `yaml:"storage"`

	Cache struct {
		Kind  string `yaml:"kind"`
		Redis struct {
			Addr   string `yaml:"addr"`
			DB     int    `yaml:"db"`
			Prefix string `yaml:"prefix"`
		} `yaml:"redis"`
		Memory struct {
			DefaultTTL string `yaml:"default_ttl"`
		} `yaml:"memory"`
	} `yaml:"cache"`

	JWT struct {
		Issuer     string `yaml:"issuer"`
		AccessTTL  string `yaml:"access_ttl"`
		RefreshTTL string `yaml:"refresh_ttl"`
	} `yaml:"jwt"`

	Register struct {
		AutoLogin bool `yaml:"auto_login"`
	} `yaml:"register"`

	Auth struct {
		AllowBearerSession bool `yaml:"allow_bearer_session"`
		Session            struct {
			CookieName string `yaml:"cookie_name"`
			Domain     string `yaml:"domain"`
			SameSite   string `yaml:"samesite"`
			Secure     bool   `yaml:"secure"`
			TTL        string `yaml:"ttl"`
		} `yaml:"session"`
		Reset struct {
			TTL       time.Duration `yaml:"ttl"`
			AutoLogin bool          `yaml:"auto_login"`
		} `yaml:"reset"`
		Verify struct {
			TTL time.Duration `yaml:"ttl"`
		} `yaml:"verify"`
		IntrospectBasicUser string `yaml:"introspect_basic_user"`
		IntrospectBasicPass string `yaml:"introspect_basic_pass"`
	} `yaml:"auth"`

	Rate struct {
		Enabled     bool   `yaml:"enabled"`
		Window      string `yaml:"window"`       // global (backward compatibility)
		MaxRequests int    `yaml:"max_requests"` // global (backward compatibility)

		// Endpoint-specific configurations
		Login struct {
			Limit  int    `yaml:"limit"`
			Window string `yaml:"window"`
		} `yaml:"login"`

		Forgot struct {
			Limit  int    `yaml:"limit"`
			Window string `yaml:"window"`
		} `yaml:"forgot"`

		// MFA endpoint configs (simple per-endpoint limits)
		MFA struct {
			Enroll struct {
				Limit  int    `yaml:"limit"`
				Window string `yaml:"window"`
			} `yaml:"enroll"`
			Verify struct {
				Limit  int    `yaml:"limit"`
				Window string `yaml:"window"`
			} `yaml:"verify"`
			Challenge struct {
				Limit  int    `yaml:"limit"`
				Window string `yaml:"window"`
			} `yaml:"challenge"`
			Disable struct {
				Limit  int    `yaml:"limit"`
				Window string `yaml:"window"`
			} `yaml:"disable"`
		} `yaml:"mfa"`
	} `yaml:"rate"`

	Flags struct {
		Migrate bool `yaml:"migrate"`
	} `yaml:"flags"`

	SMTP struct {
		Host               string `yaml:"host"`
		Port               int    `yaml:"port"`
		Username           string `yaml:"username"`
		Password           string `yaml:"password"`
		From               string `yaml:"from"`
		TLS                string `yaml:"tls"`                  // auto | starttls | ssl | none
		InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // sólo dev
	} `yaml:"smtp"`

	Email struct {
		BaseURL        string `yaml:"base_url"`
		TemplatesDir   string `yaml:"templates_dir"`
		DebugEchoLinks bool   `yaml:"debug_echo_links"`
	} `yaml:"email"`

	Security struct {
		SecretBoxMasterKey string `yaml:"secretbox_master_key"` // base64(32 bytes) for encrypting secrets in FS config
		PasswordPolicy     struct {
			MinLength     int  `yaml:"min_length"`
			RequireUpper  bool `yaml:"require_upper"`
			RequireLower  bool `yaml:"require_lower"`
			RequireDigit  bool `yaml:"require_digit"`
			RequireSymbol bool `yaml:"require_symbol"`
		} `yaml:"password_policy"`
		PasswordBlacklistPath string `yaml:"password_blacklist_path"`
	} `yaml:"security"`

	ControlPlane struct {
		FSRoot string `yaml:"fs_root"` // root directory for filesystem-based control plane
	} `yaml:"control_plane"`

	// Cluster (Fase 6 - Paso 0)
	Cluster struct {
		Mode            string            `yaml:"mode" json:"mode"` // off | embedded
		NodeID          string            `yaml:"node_id" json:"nodeId"`
		RaftAddr        string            `yaml:"raft_addr" json:"raftAddr"`
		Nodes           map[string]string `yaml:"nodes" json:"nodes"`                      // nodeID -> host:port (raft)
		LeaderRedirects map[string]string `yaml:"leader_redirects" json:"leaderRedirects"` // nodeID -> baseURL
		SnapshotEvery   int               `yaml:"snapshot_every" json:"snapshotEvery"`
		MaxLogMB        int               `yaml:"max_log_mb" json:"maxLogMb"`

		// TLS for Raft transport (optional, mTLS when enabled)
		RaftTLSEnable     bool   `yaml:"raft_tls_enable" json:"raftTlsEnable"`
		RaftTLSCertFile   string `yaml:"raft_tls_cert_file" json:"raftTlsCertFile"`
		RaftTLSKeyFile    string `yaml:"raft_tls_key_file" json:"raftTlsKeyFile"`
		RaftTLSCAFile     string `yaml:"raft_tls_ca_file" json:"raftTlsCaFile"`
		RaftTLSServerName string `yaml:"raft_tls_server_name" json:"raftTlsServerName"`
	} `yaml:"cluster" json:"cluster"`

	// ───────── Social Login Providers ─────────
	Providers struct {
		LoginCodeTTL time.Duration `yaml:"login_code_ttl"` // NUEVO: TTL para el login_code del social flow
		Google       struct {
			Enabled        bool     `yaml:"enabled"`
			ClientID       string   `yaml:"client_id"`
			ClientSecret   string   `yaml:"client_secret"`
			RedirectURL    string   `yaml:"redirect_url"` // si vacío => <jwt.issuer>/v1/auth/social/google/callback
			Scopes         []string `yaml:"scopes"`       // default: openid,email,profile
			AllowedTenants []string `yaml:"allowed_tenants"`
			AllowedClients []string `yaml:"allowed_clients"`
		} `yaml:"google"`
	} `yaml:"providers"`

	// Key rotation grace window for exposing both old (retiring) and new (active) keys
	KeyRotationGraceSeconds int `yaml:"key_rotation_grace_seconds" json:"keyRotationGraceSeconds"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	// sane defaults
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Cache.Kind == "" {
		c.Cache.Kind = "memory"
	}
	if c.Cache.Memory.DefaultTTL == "" {
		c.Cache.Memory.DefaultTTL = "2m"
	}
	if c.JWT.RefreshTTL == "" {
		c.JWT.RefreshTTL = "720h" // 30d
	}
	if c.Rate.Window == "" {
		c.Rate.Window = "1m"
	}
	if c.Rate.MaxRequests == 0 {
		c.Rate.MaxRequests = 60
	}
	// Endpoint-specific rate limit defaults
	if c.Rate.Login.Limit == 0 {
		c.Rate.Login.Limit = 10
	}
	if c.Rate.Login.Window == "" {
		c.Rate.Login.Window = "1m"
	}
	if c.Rate.Forgot.Limit == 0 {
		c.Rate.Forgot.Limit = 5
	}
	if c.Rate.Forgot.Window == "" {
		c.Rate.Forgot.Window = "10m"
	}
	// MFA defaults
	if c.Rate.MFA.Enroll.Limit == 0 {
		c.Rate.MFA.Enroll.Limit = 3
	}
	if c.Rate.MFA.Enroll.Window == "" {
		c.Rate.MFA.Enroll.Window = "10m"
	}
	if c.Rate.MFA.Verify.Limit == 0 {
		c.Rate.MFA.Verify.Limit = 10
	}
	if c.Rate.MFA.Verify.Window == "" {
		c.Rate.MFA.Verify.Window = "1m"
	}
	if c.Rate.MFA.Challenge.Limit == 0 {
		c.Rate.MFA.Challenge.Limit = 10
	}
	if c.Rate.MFA.Challenge.Window == "" {
		c.Rate.MFA.Challenge.Window = "1m"
	}
	if c.Rate.MFA.Disable.Limit == 0 {
		c.Rate.MFA.Disable.Limit = 3
	}
	if c.Rate.MFA.Disable.Window == "" {
		c.Rate.MFA.Disable.Window = "10m"
	}
	// Auth/session defaults
	if c.Auth.Session.CookieName == "" {
		c.Auth.Session.CookieName = "sid"
	}
	if c.Auth.Session.SameSite == "" {
		c.Auth.Session.SameSite = "Lax"
	}
	if c.Auth.Session.TTL == "" {
		c.Auth.Session.TTL = "12h"
	}
	// AllowBearerSession: por compat en dev
	if !c.Auth.AllowBearerSession {
		c.Auth.AllowBearerSession = true
	}
	// Email flows defaults
	if c.Auth.Reset.TTL == 0 {
		c.Auth.Reset.TTL = 60 * time.Minute
	}
	if c.Auth.Verify.TTL == 0 {
		c.Auth.Verify.TTL = 48 * time.Hour
	}
	// Password policy default
	if c.Security.PasswordPolicy.MinLength == 0 {
		c.Security.PasswordPolicy.MinLength = 10
	}
	// SMTP defaults
	if c.SMTP.TLS == "" {
		c.SMTP.TLS = "auto"
	}

	// Social defaults
	if len(c.Providers.Google.Scopes) == 0 {
		c.Providers.Google.Scopes = []string{"openid", "email", "profile"}
	}
	if c.Providers.LoginCodeTTL == 0 {
		c.Providers.LoginCodeTTL = 60 * time.Second
	}

	// Key rotation grace default
	if c.KeyRotationGraceSeconds == 0 {
		c.KeyRotationGraceSeconds = 60
	}

	// validate string durations
	if c.Storage.Postgres.ConnMaxLifetime != "" {
		if _, err := time.ParseDuration(c.Storage.Postgres.ConnMaxLifetime); err != nil {
			return nil, err
		}
	}
	if c.JWT.AccessTTL != "" {
		if _, err := time.ParseDuration(c.JWT.AccessTTL); err != nil {
			return nil, err
		}
	}
	if c.JWT.RefreshTTL != "" {
		if _, err := time.ParseDuration(c.JWT.RefreshTTL); err != nil {
			return nil, err
		}
	}
	if c.Rate.Window != "" {
		if _, err := time.ParseDuration(c.Rate.Window); err != nil {
			return nil, err
		}
	}
	// validate MFA duration strings if present
	if c.Rate.MFA.Enroll.Window != "" {
		if _, err := time.ParseDuration(c.Rate.MFA.Enroll.Window); err != nil {
			return nil, err
		}
	}
	if c.Rate.MFA.Verify.Window != "" {
		if _, err := time.ParseDuration(c.Rate.MFA.Verify.Window); err != nil {
			return nil, err
		}
	}
	if c.Rate.MFA.Challenge.Window != "" {
		if _, err := time.ParseDuration(c.Rate.MFA.Challenge.Window); err != nil {
			return nil, err
		}
	}
	if c.Rate.MFA.Disable.Window != "" {
		if _, err := time.ParseDuration(c.Rate.MFA.Disable.Window); err != nil {
			return nil, err
		}
	}
	if c.Auth.Session.TTL != "" {
		if _, err := time.ParseDuration(c.Auth.Session.TTL); err != nil {
			return nil, err
		}
	}

	// ControlPlane defaults
	if c.ControlPlane.FSRoot == "" {
		c.ControlPlane.FSRoot = "./data/hellojohn" // default for development
	}

	// Cluster defaults (Paso 0): feature flag off by default
	if strings.TrimSpace(c.Cluster.Mode) == "" {
		c.Cluster.Mode = "off"
	}
	if c.Cluster.Nodes == nil {
		c.Cluster.Nodes = map[string]string{}
	}
	if c.Cluster.LeaderRedirects == nil {
		c.Cluster.LeaderRedirects = map[string]string{}
	}

	// Overrides por env + salvaguarda prod
	c.applyEnvOverrides()

	// Optional override for rotation grace via env (KEY_ROTATION_GRACE_SECONDS)
	if v, ok := getEnvInt("KEY_ROTATION_GRACE_SECONDS"); ok {
		c.KeyRotationGraceSeconds = v
	}

	// Validation
	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Si Google.RedirectURL vacío pero tenemos issuer ⇒ autogenerar
	if c.Providers.Google.Enabled && strings.TrimSpace(c.Providers.Google.RedirectURL) == "" && strings.TrimSpace(c.JWT.Issuer) != "" {
		c.Providers.Google.RedirectURL = strings.TrimRight(c.JWT.Issuer, "/") + "/v1/auth/social/google/callback"
	}

	// Guardia dura: en prod NUNCA exponemos los links por headers.
	if strings.EqualFold(c.App.Env, "prod") {
		c.Email.DebugEchoLinks = false
	}

	// Normalizar ruta de blacklist (si relativa) respecto al directorio del YAML
	if p := strings.TrimSpace(c.Security.PasswordBlacklistPath); p != "" {
		if !filepath.IsAbs(p) {
			base := filepath.Dir(path)
			c.Security.PasswordBlacklistPath = filepath.Clean(filepath.Join(base, p))
		}
	}

	return &c, nil
}

// ---- Helpers env ----

func getEnvStr(key string) (string, bool) {
	v := os.Getenv(key)
	return v, v != ""
}
func getEnvInt(key string) (int, bool) {
	if s, ok := getEnvStr(key); ok {
		if i, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return i, true
		}
	}
	return 0, false
}
func getEnvBool(key string) (bool, bool) {
	if s, ok := getEnvStr(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
			return b, true
		}
	}
	return false, false
}
func getEnvDur(key string) (time.Duration, bool) {
	if s, ok := getEnvStr(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil {
			return d, true
		}
	}
	return 0, false
}
func getEnvCSV(key string) ([]string, bool) {
	if s, ok := getEnvStr(key); ok {
		if strings.TrimSpace(s) == "" {
			return []string{}, true
		}
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out, true
	}
	return nil, false
}

// applyEnvOverrides: pisa config.yaml con variables de entorno
// y fuerza seguridad en prod (sin X-Debug-*).
func (c *Config) applyEnvOverrides() {
	// APP
	if v, ok := getEnvStr("APP_ENV"); ok {
		c.App.Env = strings.ToLower(v)
	}

	// SERVER
	if v, ok := getEnvStr("SERVER_ADDR"); ok {
		c.Server.Addr = v
	}
	if v, ok := getEnvCSV("SERVER_CORS_ALLOWED_ORIGINS"); ok {
		c.Server.CORSAllowedOrigins = v
	}

	// STORAGE
	if v, ok := getEnvStr("STORAGE_DRIVER"); ok {
		c.Storage.Driver = v
	}
	if v, ok := getEnvStr("STORAGE_DSN"); ok {
		c.Storage.DSN = v
	}
	// Aliases for specific drivers
	if v, ok := getEnvStr("MYSQL_DSN"); ok {
		c.Storage.MySQL.DSN = v
	}
	if v, ok := getEnvStr("MONGO_URI"); ok {
		c.Storage.Mongo.URI = v
	}
	if v, ok := getEnvStr("MONGO_DATABASE"); ok {
		c.Storage.Mongo.Database = v
	}
	if v, ok := getEnvInt("POSTGRES_MAX_OPEN_CONNS"); ok {
		c.Storage.Postgres.MaxOpenConns = v
	}
	if v, ok := getEnvInt("POSTGRES_MAX_IDLE_CONNS"); ok {
		c.Storage.Postgres.MaxIdleConns = v
	}
	if v, ok := getEnvStr("POSTGRES_CONN_MAX_LIFETIME"); ok {
		// validación ya existe más arriba
		c.Storage.Postgres.ConnMaxLifetime = v
	}

	// CACHE
	if v, ok := getEnvStr("CACHE_KIND"); ok {
		c.Cache.Kind = v
	}
	if v, ok := getEnvStr("REDIS_ADDR"); ok {
		c.Cache.Redis.Addr = v
	}
	if v, ok := getEnvInt("REDIS_DB"); ok {
		c.Cache.Redis.DB = v
	}
	if v, ok := getEnvStr("REDIS_PREFIX"); ok {
		c.Cache.Redis.Prefix = v
	}
	// Support both legacy and namespaced env names
	if v, ok := getEnvStr("CACHE_MEMORY_DEFAULT_TTL"); ok {
		c.Cache.Memory.DefaultTTL = v
	} else if v, ok := getEnvStr("MEMORY_DEFAULT_TTL"); ok {
		c.Cache.Memory.DefaultTTL = v
	}

	// JWT
	if v, ok := getEnvStr("JWT_ISSUER"); ok {
		c.JWT.Issuer = v
	}
	if v, ok := getEnvStr("JWT_ACCESS_TTL"); ok {
		c.JWT.AccessTTL = v
	}
	if v, ok := getEnvStr("JWT_REFRESH_TTL"); ok {
		c.JWT.RefreshTTL = v
	}
	// Test-only overrides (useful in CI/e2e): take precedence if set
	if v, ok := getEnvStr("TEST_ACCESS_TTL"); ok {
		c.JWT.AccessTTL = v
	}
	if v, ok := getEnvStr("TEST_REFRESH_TTL"); ok {
		c.JWT.RefreshTTL = v
	}

	// REGISTER
	if v, ok := getEnvBool("REGISTER_AUTO_LOGIN"); ok {
		c.Register.AutoLogin = v
	}

	// AUTH
	if v, ok := getEnvBool("AUTH_ALLOW_BEARER_SESSION"); ok {
		c.Auth.AllowBearerSession = v
	}
	if v, ok := getEnvStr("AUTH_SESSION_COOKIE_NAME"); ok {
		c.Auth.Session.CookieName = v
	}
	if v, ok := getEnvStr("AUTH_SESSION_DOMAIN"); ok {
		c.Auth.Session.Domain = v
	}
	if v, ok := getEnvStr("AUTH_SESSION_SAMESITE"); ok {
		c.Auth.Session.SameSite = v
	}
	if v, ok := getEnvBool("AUTH_SESSION_SECURE"); ok {
		c.Auth.Session.Secure = v
	}
	if v, ok := getEnvStr("AUTH_SESSION_TTL"); ok {
		c.Auth.Session.TTL = v
	}
	if v, ok := getEnvDur("AUTH_RESET_TTL"); ok {
		c.Auth.Reset.TTL = v
	}
	if v, ok := getEnvBool("AUTH_RESET_AUTO_LOGIN"); ok {
		c.Auth.Reset.AutoLogin = v
	}
	if v, ok := getEnvDur("AUTH_VERIFY_TTL"); ok {
		c.Auth.Verify.TTL = v
	}
	// Basic auth for /oauth2/introspect (env-only in many setups, but allow YAML+ENV)
	if v, ok := getEnvStr("INTROSPECT_BASIC_USER"); ok {
		c.Auth.IntrospectBasicUser = v
	}
	if v, ok := getEnvStr("INTROSPECT_BASIC_PASS"); ok {
		c.Auth.IntrospectBasicPass = v
	}

	// RATE
	if v, ok := getEnvBool("RATE_ENABLED"); ok {
		c.Rate.Enabled = v
	}
	if v, ok := getEnvStr("RATE_WINDOW"); ok {
		c.Rate.Window = v
	}
	if v, ok := getEnvInt("RATE_MAX_REQUESTS"); ok {
		c.Rate.MaxRequests = v
	}

	// Rate limit endpoints específicos
	if v, ok := getEnvInt("RATE_LOGIN_LIMIT"); ok {
		c.Rate.Login.Limit = v
	}
	if v, ok := getEnvStr("RATE_LOGIN_WINDOW"); ok {
		c.Rate.Login.Window = v
	}
	if v, ok := getEnvInt("RATE_FORGOT_LIMIT"); ok {
		c.Rate.Forgot.Limit = v
	}
	if v, ok := getEnvStr("RATE_FORGOT_WINDOW"); ok {
		c.Rate.Forgot.Window = v
	}

	// Rate limit MFA endpoints específicos
	if v, ok := getEnvInt("RATE_MFA_ENROLL_LIMIT"); ok {
		c.Rate.MFA.Enroll.Limit = v
	}
	if v, ok := getEnvStr("RATE_MFA_ENROLL_WINDOW"); ok {
		c.Rate.MFA.Enroll.Window = v
	}
	if v, ok := getEnvInt("RATE_MFA_VERIFY_LIMIT"); ok {
		c.Rate.MFA.Verify.Limit = v
	}
	if v, ok := getEnvStr("RATE_MFA_VERIFY_WINDOW"); ok {
		c.Rate.MFA.Verify.Window = v
	}
	if v, ok := getEnvInt("RATE_MFA_CHALLENGE_LIMIT"); ok {
		c.Rate.MFA.Challenge.Limit = v
	}
	if v, ok := getEnvStr("RATE_MFA_CHALLENGE_WINDOW"); ok {
		c.Rate.MFA.Challenge.Window = v
	}
	if v, ok := getEnvInt("RATE_MFA_DISABLE_LIMIT"); ok {
		c.Rate.MFA.Disable.Limit = v
	}
	if v, ok := getEnvStr("RATE_MFA_DISABLE_WINDOW"); ok {
		c.Rate.MFA.Disable.Window = v
	}

	// FLAGS
	if v, ok := getEnvBool("FLAGS_MIGRATE"); ok {
		c.Flags.Migrate = v
	}

	// SMTP
	if v, ok := getEnvStr("SMTP_HOST"); ok {
		c.SMTP.Host = v
	}
	if v, ok := getEnvInt("SMTP_PORT"); ok {
		c.SMTP.Port = v
	}
	if v, ok := getEnvStr("SMTP_USERNAME"); ok {
		c.SMTP.Username = v
	}
	if v, ok := getEnvStr("SMTP_PASSWORD"); ok {
		c.SMTP.Password = v
	}
	if v, ok := getEnvStr("SMTP_FROM"); ok {
		c.SMTP.From = v
	}
	if v, ok := getEnvStr("SMTP_TLS"); ok {
		c.SMTP.TLS = strings.ToLower(v) // auto|starttls|ssl|none
	}
	if v, ok := getEnvBool("SMTP_INSECURE_SKIP_VERIFY"); ok {
		c.SMTP.InsecureSkipVerify = v
	}

	// EMAIL
	if v, ok := getEnvStr("EMAIL_BASE_URL"); ok {
		c.Email.BaseURL = v
	}
	if v, ok := getEnvStr("EMAIL_TEMPLATES_DIR"); ok {
		c.Email.TemplatesDir = v
	}
	if v, ok := getEnvBool("EMAIL_DEBUG_LINKS"); ok {
		c.Email.DebugEchoLinks = v
	}

	// SECURITY
	if v, ok := getEnvInt("SECURITY_PASSWORD_POLICY_MIN_LENGTH"); ok {
		c.Security.PasswordPolicy.MinLength = v
	}
	if v, ok := getEnvBool("SECURITY_PASSWORD_POLICY_REQUIRE_UPPER"); ok {
		c.Security.PasswordPolicy.RequireUpper = v
	}
	if v, ok := getEnvBool("SECURITY_PASSWORD_POLICY_REQUIRE_LOWER"); ok {
		c.Security.PasswordPolicy.RequireLower = v
	}
	if v, ok := getEnvBool("SECURITY_PASSWORD_POLICY_REQUIRE_DIGIT"); ok {
		c.Security.PasswordPolicy.RequireDigit = v
	}
	if v, ok := getEnvBool("SECURITY_PASSWORD_POLICY_REQUIRE_SYMBOL"); ok {
		c.Security.PasswordPolicy.RequireSymbol = v
	}
	if v, ok := getEnvStr("SECURITY_PASSWORD_BLACKLIST_PATH"); ok {
		c.Security.PasswordBlacklistPath = strings.TrimSpace(v)
	}

	// ───── Providers (Social) ─────
	// TTL del login_code del flujo social
	if d, ok := getEnvDur("SOCIAL_LOGIN_CODE_TTL"); ok {
		c.Providers.LoginCodeTTL = d
	}
	// GOOGLE
	if v, ok := getEnvBool("GOOGLE_ENABLED"); ok {
		c.Providers.Google.Enabled = v
	}
	if v, ok := getEnvStr("GOOGLE_CLIENT_ID"); ok {
		c.Providers.Google.ClientID = v
	}
	if v, ok := getEnvStr("GOOGLE_CLIENT_SECRET"); ok {
		c.Providers.Google.ClientSecret = v
	}
	if v, ok := getEnvStr("GOOGLE_REDIRECT_URL"); ok {
		c.Providers.Google.RedirectURL = v
	}
	if v, ok := getEnvCSV("GOOGLE_SCOPES"); ok && len(v) > 0 {
		c.Providers.Google.Scopes = v
	}
	if v, ok := getEnvCSV("GOOGLE_ALLOWED_TENANTS"); ok {
		c.Providers.Google.AllowedTenants = v
	}
	if v, ok := getEnvCSV("GOOGLE_ALLOWED_CLIENTS"); ok {
		c.Providers.Google.AllowedClients = v
	}

	// CONTROL_PLANE
	if v, ok := getEnvStr("CONTROL_PLANE_FS_ROOT"); ok {
		c.ControlPlane.FSRoot = v
	}

	// SECURITY - SecretBox Master Key
	if v, ok := getEnvStr("SECRETBOX_MASTER_KEY"); ok {
		c.Security.SecretBoxMasterKey = v
	}

	// ───── Cluster (Paso 0) ─────
	// CLUSTER_MODE=off|embedded (default off)
	if v, ok := getEnvStr("CLUSTER_MODE"); ok {
		c.Cluster.Mode = strings.ToLower(strings.TrimSpace(v))
	}
	// NODE_ID, RAFT_ADDR
	if v, ok := getEnvStr("NODE_ID"); ok {
		c.Cluster.NodeID = strings.TrimSpace(v)
	}
	if v, ok := getEnvStr("RAFT_ADDR"); ok {
		c.Cluster.RaftAddr = strings.TrimSpace(v)
	}
	// CLUSTER_NODES="n1=127.0.0.1:8201;n2=127.0.0.1:8202"
	if m, ok := getEnvKVList("CLUSTER_NODES", ";"); ok {
		if c.Cluster.Nodes == nil {
			c.Cluster.Nodes = map[string]string{}
		}
		for k, v := range m {
			c.Cluster.Nodes[k] = v
		}
	}
	// LEADER_REDIRECTS="n1=http://127.0.0.1:8081;n2=http://127.0.0.1:8082"
	if m, ok := getEnvKVList("LEADER_REDIRECTS", ";"); ok {
		if c.Cluster.LeaderRedirects == nil {
			c.Cluster.LeaderRedirects = map[string]string{}
		}
		for k, v := range m {
			c.Cluster.LeaderRedirects[k] = v
		}
	}
	if v, ok := getEnvInt("RAFT_SNAPSHOT_EVERY"); ok {
		c.Cluster.SnapshotEvery = v
	}
	if v, ok := getEnvInt("RAFT_MAX_LOG_MB"); ok {
		c.Cluster.MaxLogMB = v
	}

	// Raft TLS (optional)
	if v, ok := getEnvBool("RAFT_TLS_ENABLE"); ok {
		c.Cluster.RaftTLSEnable = v
	}
	if v, ok := getEnvStr("RAFT_TLS_CERT_FILE"); ok {
		c.Cluster.RaftTLSCertFile = v
	} else if v, ok := getEnvStr("RAFT_TLS_CERT"); ok {
		// fallback alias
		c.Cluster.RaftTLSCertFile = v
	}
	if v, ok := getEnvStr("RAFT_TLS_KEY_FILE"); ok {
		c.Cluster.RaftTLSKeyFile = v
	} else if v, ok := getEnvStr("RAFT_TLS_KEY"); ok {
		c.Cluster.RaftTLSKeyFile = v
	}
	if v, ok := getEnvStr("RAFT_TLS_CA_FILE"); ok {
		c.Cluster.RaftTLSCAFile = v
	} else if v, ok := getEnvStr("RAFT_TLS_CA"); ok {
		c.Cluster.RaftTLSCAFile = v
	}
	if v, ok := getEnvStr("RAFT_TLS_SERVER_NAME"); ok {
		c.Cluster.RaftTLSServerName = v
	}
}

// Validate performs validation of critical configuration values
// Note: SECRETBOX validation is now in cmd/service since only the service uses FS
func (c *Config) Validate() error {
	// General config validation can go here (if needed)
	return nil
}

// parse env of form "k1=v1<sep>k2=v2" into map
func parseKVList(s, sep string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]string{}
	}
	items := strings.Split(s, sep)
	out := make(map[string]string, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		// split at first '='
		if i := strings.IndexRune(it, '='); i > 0 {
			k := strings.TrimSpace(it[:i])
			v := strings.TrimSpace(it[i+1:])
			if k != "" && v != "" {
				out[k] = v
			}
		}
	}
	return out
}

func getEnvKVList(key, sep string) (map[string]string, bool) {
	if s, ok := getEnvStr(key); ok {
		return parseKVList(s, sep), true
	}
	return nil, false
}
