package session

import (
	"context"
	"net/http"
	"time"

	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/session"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"go.uber.org/zap"
)

// Cache defines the cache interface for session storage.
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
}

// SessionLogoutService defines operations for session logout.
type SessionLogoutService interface {
	Logout(ctx context.Context, sessionID string) error
	BuildDeletionCookie(config dto.SessionLogoutConfig) *http.Cookie
}

// SessionLogoutDeps contains dependencies for the session logout service.
type SessionLogoutDeps struct {
	Cache  Cache
	Config dto.SessionLogoutConfig
}

type sessionLogoutService struct {
	deps SessionLogoutDeps
}

// NewSessionLogoutService creates a new SessionLogoutService.
func NewSessionLogoutService(deps SessionLogoutDeps) SessionLogoutService {
	return &sessionLogoutService{deps: deps}
}

// Logout invalidates a session by deleting it from the cache.
func (s *sessionLogoutService) Logout(ctx context.Context, sessionID string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("session.logout"),
		logger.Op("Logout"),
	)

	if sessionID == "" {
		log.Debug("no session ID provided, skipping cache delete")
		return nil
	}

	// Build cache key using same hash as session_login
	key := "sid:" + tokens.SHA256Base64URL(sessionID)

	if err := s.deps.Cache.Delete(key); err != nil {
		// Log but don't fail - logout should be best-effort
		log.Debug("failed to delete session from cache", logger.Err(err), zap.String("key_prefix", "sid:"))
	} else {
		log.Debug("session deleted from cache")
	}

	return nil
}

// BuildDeletionCookie creates a cookie that expires immediately to clear the session cookie.
func (s *sessionLogoutService) BuildDeletionCookie(config dto.SessionLogoutConfig) *http.Cookie {
	cookieName := config.CookieName
	if cookieName == "" {
		cookieName = "sid"
	}

	sameSite := http.SameSiteLaxMode
	switch config.SameSite {
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	case "Lax":
		sameSite = http.SameSiteLaxMode
	}

	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   config.CookieDomain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   config.Secure,
		SameSite: sameSite,
	}
}
