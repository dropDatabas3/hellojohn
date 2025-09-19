package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
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

		rev, ok := c.Store.(revoker)
		if !ok {
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta revocación masiva", 2502)
			return
		}

		if err := rev.RevokeAllRefreshTokens(r.Context(), target, strings.TrimSpace(req.ClientID)); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "revocation_failed", "no se pudo revocar", 2503)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
