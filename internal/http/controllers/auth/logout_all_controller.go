package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/auth"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/errors"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	svc "github.com/dropDatabas3/hellojohn/internal/http/services/auth"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

const maxLogoutBodySize = 4 * 1024 // 4KB

// LogoutController handles POST /v2/auth/logout and /v2/auth/logout-all
type LogoutController struct {
	service svc.LogoutService
}

// NewLogoutController creates a new controller for logout.
func NewLogoutController(service svc.LogoutService) *LogoutController {
	return &LogoutController{service: service}
}

// Logout handles POST /v2/auth/logout (single refresh token revocation)
func (c *LogoutController) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("LogoutController.Logout"))

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLogoutBodySize)
	defer r.Body.Close()

	var req dto.LogoutRequest
	ct := strings.ToLower(r.Header.Get("Content-Type"))

	switch {
	case strings.Contains(ct, "application/json"):
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperrors.WriteError(w, httperrors.ErrInvalidJSON)
			return
		}

	case strings.Contains(ct, "application/x-www-form-urlencoded"):
		if err := r.ParseForm(); err != nil {
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("invalid form"))
			return
		}
		req.TenantID = r.FormValue("tenant_id")
		req.ClientID = r.FormValue("client_id")
		req.RefreshToken = r.FormValue("refresh_token")

	default:
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("unsupported content type"))
		return
	}

	tenantSlug := helpers.ResolveTenantSlug(r)

	err := c.service.Logout(ctx, req, tenantSlug)
	if err != nil {
		log.Debug("logout failed", logger.Err(err))
		writeLogoutError(w, err)
		return
	}

	// Success: 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll handles POST /v2/auth/logout-all (mass token revocation)
func (c *LogoutController) LogoutAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("LogoutController.LogoutAll"))

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLogoutBodySize)
	defer r.Body.Close()

	var req dto.LogoutAllRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	tenantSlug := helpers.ResolveTenantSlug(r)

	err := c.service.LogoutAll(ctx, req, tenantSlug)
	if err != nil {
		log.Debug("logout-all failed", logger.Err(err))
		writeLogoutError(w, err)
		return
	}

	// Success: 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// ─── Error Mapping ───

func writeLogoutError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, svc.ErrLogoutMissingFields):
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("campos requeridos faltantes"))

	case errors.Is(err, svc.ErrLogoutInvalidClient):
		httperrors.WriteError(w, httperrors.ErrUnauthorized.WithDetail("client_id no coincide"))

	case errors.Is(err, svc.ErrLogoutNoDatabase):
		httperrors.WriteError(w, httperrors.ErrServiceUnavailable.WithDetail("base de datos no disponible"))

	case errors.Is(err, svc.ErrLogoutNotSupported):
		httperrors.WriteError(w, httperrors.ErrNotImplemented.WithDetail("revocación masiva no soportada"))

	case errors.Is(err, svc.ErrLogoutFailed):
		httperrors.WriteError(w, httperrors.ErrInternalServerError.WithDetail("error al revocar tokens"))

	default:
		httperrors.WriteError(w, httperrors.ErrInternalServerError)
	}
}
