package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/audit"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

type AdminUsersHandler struct{ c *app.Container }

func NewAdminUsersHandler(c *app.Container) *AdminUsersHandler { return &AdminUsersHandler{c: c} }

type adminUserReq struct {
	UserID string `json:"user_id"`
	Reason string `json:"reason,omitempty"`
}

func (h *AdminUsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c == nil || h.c.Store == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store requerido", 3800)
		return
	}
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
		return
	}

	var body adminUserReq
	if !httpx.ReadJSON(w, r, &body) {
		return
	}
	body.UserID = strings.TrimSpace(body.UserID)
	body.Reason = strings.TrimSpace(body.Reason)
	if body.UserID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id requerido", 3801)
		return
	}

	// Who performs the action (for audit fields)
	by := ""
	if cl := httpx.GetClaims(r.Context()); cl != nil {
		if sub, _ := cl["sub"].(string); sub != "" {
			by = sub
		}
	}

	switch r.URL.Path {
	case "/v1/admin/users/disable":
		if err := h.c.Store.DisableUser(r.Context(), body.UserID, by, body.Reason); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "disable_failed", err.Error(), 3803)
			return
		}
		// Best-effort: revocar todos los refresh del usuario
		if rev, ok := h.c.Store.(interface {
			RevokeAllRefreshByUser(context.Context, string) (int, error)
		}); ok {
			_, _ = rev.RevokeAllRefreshByUser(r.Context(), body.UserID)
		} else {
			_ = h.c.Store.RevokeAllRefreshTokens(r.Context(), body.UserID, "")
		}
		// Audit
		audit.Log(r.Context(), "admin_user_disabled", map[string]any{
			"by": by, "user_id": body.UserID, "reason": body.Reason,
		})
		w.WriteHeader(http.StatusNoContent)
		return

	case "/v1/admin/users/enable":
		if err := h.c.Store.EnableUser(r.Context(), body.UserID, by); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "enable_failed", err.Error(), 3805)
			return
		}
		audit.Log(r.Context(), "admin_user_enabled", map[string]any{
			"by": by, "user_id": body.UserID,
		})
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Fallback 404
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
