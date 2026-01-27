package session

import (
	"net/http"
	"net/url"
	"strings"

	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/session"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/errors"
	svc "github.com/dropDatabas3/hellojohn/internal/http/services/session"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"go.uber.org/zap"
)

// SessionLogoutController handles POST /v2/session/logout.
type SessionLogoutController struct {
	service svc.SessionLogoutService
	config  dto.SessionLogoutConfig
}

// NewSessionLogoutController creates a new session logout controller.
func NewSessionLogoutController(service svc.SessionLogoutService, config dto.SessionLogoutConfig) *SessionLogoutController {
	return &SessionLogoutController{
		service: service,
		config:  config,
	}
}

// Logout handles the session logout request.
// Clears session from cache and sets deletion cookie.
// Optionally redirects to return_to if host is in allowlist.
func (c *SessionLogoutController) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("SessionLogoutController.Logout"))

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	// Get session cookie
	cookieName := c.config.CookieName
	if cookieName == "" {
		cookieName = "sid"
	}

	ck, err := r.Cookie(cookieName)
	if err == nil && ck != nil && strings.TrimSpace(ck.Value) != "" {
		// Delete session from cache
		if err := c.service.Logout(ctx, ck.Value); err != nil {
			log.Debug("logout service error", logger.Err(err))
		}

		// Set deletion cookie to clear browser
		delCookie := c.service.BuildDeletionCookie(c.config)
		http.SetCookie(w, delCookie)
	}

	// Handle optional return_to redirect
	returnTo := r.URL.Query().Get("return_to")
	if returnTo != "" {
		if c.isAllowedRedirect(returnTo, log) {
			http.Redirect(w, r, returnTo, http.StatusSeeOther)
			return
		}
	}

	// Default: 204 No Content
	w.WriteHeader(http.StatusNoContent)
	log.Debug("session logout completed")
}

// isAllowedRedirect checks if the return_to URL is allowed.
func (c *SessionLogoutController) isAllowedRedirect(returnTo string, log *zap.Logger) bool {
	u, err := url.Parse(returnTo)
	if err != nil {
		return false
	}

	// Must be absolute URL
	if u.Scheme == "" || u.Host == "" {
		return false
	}

	// Check against allowlist
	host := strings.ToLower(u.Hostname())
	if c.config.AllowedHosts != nil && c.config.AllowedHosts[host] {
		log.Debug("redirect allowed", zap.String("host", host))
		return true
	}

	log.Debug("redirect not allowed", zap.String("host", host))
	return false
}
