package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
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
	} `yaml:"auth"`

	Rate struct {
		Enabled     bool   `yaml:"enabled"`
		Window      string `yaml:"window"`
		MaxRequests int    `yaml:"max_requests"`
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
		PasswordPolicy struct {
			MinLength     int  `yaml:"min_length"`
			RequireUpper  bool `yaml:"require_upper"`
			RequireLower  bool `yaml:"require_lower"`
			RequireDigit  bool `yaml:"require_digit"`
			RequireSymbol bool `yaml:"require_symbol"`
		} `yaml:"password_policy"`
	} `yaml:"security"`
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
	if c.Auth.Session.TTL != "" {
		if _, err := time.ParseDuration(c.Auth.Session.TTL); err != nil {
			return nil, err
		}
	}
	return &c, nil
}
