package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

// NewProfileHandler exposes a real "whoami"/profile endpoint for UI/CLI use.
// Must be wrapped by RequireAuth and a scope check (profile:read).
func NewProfileHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		// Claims are set by RequireAuth
		cl := httpx.GetClaims(r.Context())
		if cl == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "missing_claims", "no claims in context", 4012)
			return
		}
		sub := ""
		if v, ok := cl["sub"].(string); ok {
			sub = strings.TrimSpace(v)
		}
		if sub == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "sub faltante", 4013)
			return
		}

		u, err := c.Store.GetUserByID(r.Context(), sub)
		if err != nil || u == nil {
			httpx.WriteError(w, http.StatusNotFound, "user_not_found", "usuario no encontrado", 2401)
			return
		}

		// Multi-tenant guard: if token has tid claim, ensure it matches the user's tenant
		if tidRaw, ok := cl["tid"].(string); ok && strings.TrimSpace(tidRaw) != "" {
			if !strings.EqualFold(strings.TrimSpace(tidRaw), strings.TrimSpace(u.TenantID)) {
				httpx.WriteError(w, http.StatusForbidden, "forbidden_tenant", "tenant mismatch", 2402)
				return
			}
		}

		// Build a safe, useful profile payload
		given := ""
		family := ""
		name := ""
		picture := ""
		if u.Metadata != nil {
			if s, ok := u.Metadata["given_name"].(string); ok {
				given = strings.TrimSpace(s)
			}
			if s, ok := u.Metadata["family_name"].(string); ok {
				family = strings.TrimSpace(s)
			}
			if s, ok := u.Metadata["name"].(string); ok {
				name = strings.TrimSpace(s)
			}
			if s, ok := u.Metadata["picture"].(string); ok {
				picture = strings.TrimSpace(s)
			}
		}
		if name == "" && (given != "" || family != "") {
			name = strings.TrimSpace(strings.TrimSpace(given + " " + family))
		}

		// TODO: persist and return real UpdatedAt (user.UpdatedAt) when available
		resp := map[string]any{
			"sub":            u.ID,
			"email":          u.Email,
			"email_verified": u.EmailVerified,
			"name":           name,
			"given_name":     given,
			"family_name":    family,
			"picture":        picture,
			"updated_at":     u.CreatedAt.Unix(), // best-effort; replace with updated_at when available
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
