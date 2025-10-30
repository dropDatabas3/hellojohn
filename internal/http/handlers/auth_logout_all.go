package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
)

func NewAuthLogoutAllHandler(c *app.Container) http.HandlerFunc {
	// Interface opcional para no cambiar core.Repository
	type revoker interface {
		RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 2500)
			return
		}
		var req struct {
			UserID   string `json:"user_id,omitempty"`
			ClientID string `json:"client_id,omitempty"`
		}
		if !httpx.ReadJSON(w, r, &req) {
			return
		}

		target := strings.TrimSpace(req.UserID)
		if target == "" {
			httpx.WriteError(w, http.StatusBadRequest, "user_id_required", "falta user_id", 2501)
			return
		}

		// Prefer per-tenant repository if available
		if c != nil && c.TenantSQLManager != nil {
			slug := helpers.ResolveTenantSlug(r)
			repo, err := helpers.OpenTenantRepo(r.Context(), c.TenantSQLManager, slug)
			if err != nil {
				if errors.Is(err, tenantsql.ErrNoDBForTenant) {
					httpx.WriteTenantDBMissing(w)
					return
				}
				httpx.WriteTenantDBError(w, err.Error())
				return
			}
			if rv, ok := any(repo).(revoker); ok {
				if err := rv.RevokeAllRefreshTokens(r.Context(), target, strings.TrimSpace(req.ClientID)); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "revocation_failed", "no se pudo revocar", 2503)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta revocación masiva", 2502)
			return
		}
		httpx.WriteTenantDBError(w, "tenant manager not initialized")
	}
}
