package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/rate"
)

// --- Config Types ---

type LoginRateConfig struct {
	Limit  int           `json:"limit"`
	Window time.Duration `json:"window"`
}

type ForgotRateConfig struct {
	Limit  int           `json:"limit"`
	Window time.Duration `json:"window"`
}

// MFA endpoints share a simple rate config (per endpoint)
type MFARateConfig struct {
	Limit  int           `json:"limit"`
	Window time.Duration `json:"window"`
}

// --- MultiLimiter Interface ---

// MultiLimiter permite usar diferentes límites por endpoint
// manteniendo compatibilidad con rate.Limiter existente
type MultiLimiter interface {
	AllowWithLimits(ctx context.Context, key string, limit int, window time.Duration) (rate.Result, error)
}

// --- IP Extraction (igual que en middleware) ---

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// --- Respuesta de error JSON mínima (para evitar importar internal/http) ---

func writeJSONError(w http.ResponseWriter, status int, code, desc string, appCode int) {
	type payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
		ErrorCode        int    `json:"error_code,omitempty"`
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload{
		Error:            code,
		ErrorDescription: desc,
		ErrorCode:        appCode,
	})
}

// --- Headers estándar completos ---

func setRateHeaders(w http.ResponseWriter, limit, remaining int, resetAt time.Time, retryAfter *time.Duration) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

	if retryAfter != nil && *retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
	}
}

// --- Core Enforcement Logic ---

func enforceWithKey(w http.ResponseWriter, r *http.Request, lim MultiLimiter, limit int, window time.Duration, key string) bool {
	// Fail-open si limiter ausente o configuración inválida
	if lim == nil || limit <= 0 || window <= 0 {
		return true
	}

	res, err := lim.AllowWithLimits(r.Context(), key, limit, window)
	if err != nil {
		// Fail-open en caso de error de backend (Redis down, etc.)
		return true
	}

	// Calcular resetAt basado en window truncation
	now := time.Now().UTC()
	windowStart := now.Truncate(window)
	resetAt := windowStart.Add(window)

	if res.Allowed {
		setRateHeaders(w, limit, int(res.Remaining), resetAt, nil)
		return true
	}

	// Rate limited: calcular retry-after
	retryAfter := time.Until(resetAt)
	if retryAfter < 0 {
		retryAfter = window // fallback si cálculo falla
	}

	setRateHeaders(w, limit, 0, resetAt, &retryAfter)
	writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "demasiadas solicitudes", 1401)
	return false
}

// --- Semantic Wrappers ---

// EnforceLoginLimit aplica rate limit semántico para login
func EnforceLoginLimit(w http.ResponseWriter, r *http.Request, lim MultiLimiter, cfg LoginRateConfig, tenantID, email string) bool {
	ip := clientIP(r)
	email = strings.ToLower(strings.TrimSpace(email))
	key := fmt.Sprintf("login:%s:%s:%s", tenantID, ip, email)

	return enforceWithKey(w, r, lim, cfg.Limit, cfg.Window, key)
}

// EnforceForgotLimit aplica rate limit semántico para forgot password
func EnforceForgotLimit(w http.ResponseWriter, r *http.Request, lim MultiLimiter, cfg ForgotRateConfig, tenantID, email string) bool {
	ip := clientIP(r)
	email = strings.ToLower(strings.TrimSpace(email))
	key := fmt.Sprintf("forgot:%s:%s:%s", tenantID, ip, email)

	return enforceWithKey(w, r, lim, cfg.Limit, cfg.Window, key)
}

// EnforceMFAVerifyLimit aplica rate limit por usuario+IP para verificación de código
func EnforceMFAVerifyLimit(w http.ResponseWriter, r *http.Request, lim MultiLimiter, cfg MFARateConfig, userID string) bool {
	ip := clientIP(r)
	key := fmt.Sprintf("mfa:verify:%s:%s", userID, ip)
	return enforceWithKey(w, r, lim, cfg.Limit, cfg.Window, key)
}

// EnforceMFAEnrollLimit aplica rate limit por usuario+IP para enrolamiento
func EnforceMFAEnrollLimit(w http.ResponseWriter, r *http.Request, lim MultiLimiter, cfg MFARateConfig, userID string) bool {
	ip := clientIP(r)
	key := fmt.Sprintf("mfa:enroll:%s:%s", userID, ip)
	return enforceWithKey(w, r, lim, cfg.Limit, cfg.Window, key)
}

// EnforceMFAChallengeLimit aplica rate limit por usuario+IP para el challenge (mfa_token)
func EnforceMFAChallengeLimit(w http.ResponseWriter, r *http.Request, lim MultiLimiter, cfg MFARateConfig, userID string) bool {
	ip := clientIP(r)
	key := fmt.Sprintf("mfa:challenge:%s:%s", userID, ip)
	return enforceWithKey(w, r, lim, cfg.Limit, cfg.Window, key)
}

// EnforceMFADisableLimit aplica rate limit por usuario+IP para deshabilitar MFA
func EnforceMFADisableLimit(w http.ResponseWriter, r *http.Request, lim MultiLimiter, cfg MFARateConfig, userID string) bool {
	ip := clientIP(r)
	key := fmt.Sprintf("mfa:disable:%s:%s", userID, ip)
	return enforceWithKey(w, r, lim, cfg.Limit, cfg.Window, key)
}
