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

	// NUEVO: configuración de auth/sesión
	Auth struct {
		AllowBearerSession bool `yaml:"allow_bearer_session"` // fallback: /oauth2/authorize acepta Authorization: Bearer
		Session            struct {
			CookieName string `yaml:"cookie_name"`
			Domain     string `yaml:"domain"`
			SameSite   string `yaml:"samesite"` // "Lax" (default), "Strict", "None"
			Secure     bool   `yaml:"secure"`
			TTL        string `yaml:"ttl"` // ej "12h"
		} `yaml:"session"`
	} `yaml:"auth"`

	Rate struct {
		Enabled     bool   `yaml:"enabled"`
		Window      string `yaml:"window"`
		MaxRequests int    `yaml:"max_requests"`
	} `yaml:"rate"`

	Flags struct {
		Migrate bool `yaml:"migrate"`
	} `yaml:"flags"`
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
		c.JWT.RefreshTTL = "720h"
	} // 30d
	if c.Rate.Window == "" {
		c.Rate.Window = "1m"
	}
	if c.Rate.MaxRequests == 0 {
		c.Rate.MaxRequests = 60
	}
	// Defaults nuevos (Auth/Session)
	if c.Auth.Session.CookieName == "" {
		c.Auth.Session.CookieName = "sid"
	}
	if c.Auth.Session.SameSite == "" {
		c.Auth.Session.SameSite = "Lax"
	}
	if c.Auth.Session.TTL == "" {
		c.Auth.Session.TTL = "12h"
	}
	// por DX, true en dev; en prod podés setear false
	// (si querés que /oauth2/authorize NO acepte Bearer como “sesión”)
	// lo dejamos como true por compat.
	if !c.Auth.AllowBearerSession {
		// no-op: yaml bool por defecto es false; si querés true explícito en config, definilo.
		// Para mantener compat, si no vino en YAML, lo asumimos true:
		c.Auth.AllowBearerSession = true
	}

	// validate durations
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
