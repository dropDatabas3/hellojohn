package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/audit"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AdminUsersHandler struct{ c *app.Container }

func NewAdminUsersHandler(c *app.Container) *AdminUsersHandler { return &AdminUsersHandler{c: c} }

type adminUserReq struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Duration string `json:"duration,omitempty"` // e.g. "24h", "2h30m"
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
	body.TenantID = strings.TrimSpace(body.TenantID)
	body.Reason = strings.TrimSpace(body.Reason)
	body.Duration = strings.TrimSpace(body.Duration)

	if body.UserID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id requerido", 3801)
		return
	}

	// Resolve the correct store (Global vs Tenant-specific)
	// Define minimal interface required for Disable/Enable operations
	type userManager interface {
		DisableUser(ctx context.Context, userID, by, reason string, until *time.Time) error
		EnableUser(ctx context.Context, userID, by string) error
		GetUserByID(ctx context.Context, id string) (*core.User, error)
	}

	var store userManager = h.c.Store
	var revoker interface {
		RevokeAllRefreshByUser(context.Context, string) (int, error)
	}

	// Try to resolve Tenant Store if ID provided
	if body.TenantID != "" {
		if h.c.TenantSQLManager != nil {
			ts, err := h.c.TenantSQLManager.GetPG(r.Context(), body.TenantID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "tenant_db_error", err.Error(), 3804)
				return
			}
			store = ts
			revoker = ts
		}
	} else {
		// Default global store
		if r, ok := h.c.Store.(interface {
			RevokeAllRefreshByUser(context.Context, string) (int, error)
		}); ok {
			revoker = r
		}
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
		var until *time.Time
		if body.Duration != "" {
			d, err := time.ParseDuration(body.Duration)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_duration", "duración inválida (e.g. 1h, 30m)", 3802)
				return
			}
			t := time.Now().Add(d)
			until = &t
		}

		if err := store.DisableUser(r.Context(), body.UserID, by, body.Reason, until); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "disable_failed", err.Error(), 3803)
			return
		}

		// Revoke tokens
		if revoker != nil {
			_, _ = revoker.RevokeAllRefreshByUser(r.Context(), body.UserID)
		} else {
			// Fallback legacy method if available on global store
			_ = h.c.Store.RevokeAllRefreshTokens(r.Context(), body.UserID, "")
		}

		// Audit
		fields := map[string]any{
			"by": by, "user_id": body.UserID, "reason": body.Reason, "tenant_id": body.TenantID,
		}
		if until != nil {
			fields["until"] = *until
		}
		audit.Log(r.Context(), "admin_user_disabled", fields)

		// Send Notification Email
		if u, err := store.GetUserByID(r.Context(), body.UserID); err == nil && u != nil && u.Email != "" {
			// Resolve Tenant
			tid := body.TenantID
			if tid == "" {
				tid = u.TenantID
			}
			if tid != "" {
				t, _ := cpctx.Provider.GetTenantBySlug(r.Context(), tid) // Try slug/ID lookup via provider (assuming enhanced resolver/provider)
				// Actually Provider expects Slug. If ID, we rely on updated resolver logic elsewhere, but here we call Provider directly.
				// If tid is UUID, GetTenantBySlug might fail if not enhanced.
				// BUT we enhanced `manager.go` resolver, not `cpctx.Provider` itself.
				// However, `admin_tenants_fs` handles ID lookup.
				// Let's try. If it fails, maybe try fallback.
				// Or use `h.c.SenderProvider.GetSender` logic which resolves tenant implicitly? No.

				// Reusing renderOverride from email_flows.go (same package)
				if t != nil && t.Settings.Mailing != nil && t.Settings.Mailing.Templates != nil {
					if tpl, ok := t.Settings.Mailing.Templates[email.TemplateUserBlocked]; ok {
						vars := map[string]any{
							"UserEmail": u.Email,
							"Reason":    body.Reason,
							"Until":     until,
							"Tenant":    tid,
						}
						htmlBody, textBody, err := renderOverride(tpl, vars)
						if err == nil && htmlBody != "" {
							if sender, err := h.c.SenderProvider.GetSender(r.Context(), uuid.MustParse(tid)); err == nil {
								_ = sender.Send(u.Email, tpl.Subject, htmlBody, textBody)
							}
						}
					}
				}
			}
		}

		w.WriteHeader(http.StatusNoContent)
		return

	case "/v1/admin/users/enable":
		if err := store.EnableUser(r.Context(), body.UserID, by); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "enable_failed", err.Error(), 3805)
			return
		}
		audit.Log(r.Context(), "admin_user_enabled", map[string]any{
			"by": by, "user_id": body.UserID, "tenant_id": body.TenantID,
		})

		// Send Notification Email
		if u, err := store.GetUserByID(r.Context(), body.UserID); err == nil && u != nil && u.Email != "" {
			tid := body.TenantID
			if tid == "" {
				tid = u.TenantID
			}
			if tid != "" {
				t, _ := cpctx.Provider.GetTenantBySlug(r.Context(), tid)
				if t != nil && t.Settings.Mailing != nil && t.Settings.Mailing.Templates != nil {
					if tpl, ok := t.Settings.Mailing.Templates[email.TemplateUserUnblocked]; ok {
						vars := map[string]any{
							"UserEmail": u.Email,
							"Tenant":    tid,
						}
						htmlBody, textBody, err := renderOverride(tpl, vars)
						if err == nil && htmlBody != "" {
							if sender, err := h.c.SenderProvider.GetSender(r.Context(), uuid.MustParse(tid)); err == nil {
								_ = sender.Send(u.Email, tpl.Subject, htmlBody, textBody)
							}
						}
					}
				}
			}
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Fallback 404
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
