package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// ---- Repos opcionales (type assertions) ----

type rbacReadRepo interface {
	GetUserRoles(ctxCtx interface{ Done() <-chan struct{} }, userID string) ([]string, error)
	GetUserPermissions(ctxCtx interface{ Done() <-chan struct{} }, userID string) ([]string, error)
}

type rbacWriteRepo interface {
	AssignUserRoles(ctxCtx interface{ Done() <-chan struct{} }, userID string, add []string) error
	RemoveUserRoles(ctxCtx interface{ Done() <-chan struct{} }, userID string, remove []string) error
	GetRolePerms(ctxCtx interface{ Done() <-chan struct{} }, tenantID, role string) ([]string, error)
	AddRolePerms(ctxCtx interface{ Done() <-chan struct{} }, tenantID, role string, add []string) error
	RemoveRolePerms(ctxCtx interface{ Done() <-chan struct{} }, tenantID, role string, remove []string) error
}

// ---- helpers ----

func parseBearerTenantID(iss *jwtx.Issuer, r *http.Request) (string, error) {
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
		return "", errors.New("missing bearer")
	}
	raw := strings.TrimSpace(ah[len("Bearer "):])
	tk, err := jwt.Parse(raw, iss.Keyfunc(), jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer(iss.Iss))
	if err != nil || !tk.Valid {
		return "", errors.New("invalid bearer")
	}
	if c, ok := tk.Claims.(jwt.MapClaims); ok {
		if tid, _ := c["tid"].(string); tid != "" {
			return tid, nil
		}
	}
	return "", errors.New("no tid")
}

func dedupTrim(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// ========= /v1/admin/rbac/users/{userID}/roles (GET/POST) =========

type rbacUserRolesPayload struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

func AdminRBACUsersRolesHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Path esperado: /v1/admin/rbac/users/{userID}/roles
		const pfx = "/v1/admin/rbac/users/"
		if !strings.HasPrefix(r.URL.Path, pfx) || !strings.HasSuffix(r.URL.Path, "/roles") {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "ruta no encontrada", 9401)
			return
		}
		tail := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, pfx), "/roles")
		userID := strings.Trim(tail, "/")
		if _, err := uuid.Parse(userID); err != nil || userID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id inválido", 9402)
			return
		}

		rr, okR := c.Store.(rbacReadRepo)
		if !okR {
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta RBAC read", 9403)
			return
		}

		switch r.Method {
		case http.MethodGet:
			roles, err := rr.GetUserRoles(r.Context(), userID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9404)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"user_id": userID,
				"roles":   roles,
			})
			return

		case http.MethodPost:
			rwr, okW := c.Store.(rbacWriteRepo)
			if !okW {
				httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta RBAC write", 9405)
				return
			}
			var p rbacUserRolesPayload
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "bad_json", "payload inválido", 9406)
				return
			}
			add := dedupTrim(p.Add)
			rm := dedupTrim(p.Remove)
			if len(add) > 0 {
				if err := rwr.AssignUserRoles(r.Context(), userID, add); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9407)
					return
				}
			}
			if len(rm) > 0 {
				if err := rwr.RemoveUserRoles(r.Context(), userID, rm); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9408)
					return
				}
			}
			roles, err := rr.GetUserRoles(r.Context(), userID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9409)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"user_id": userID,
				"roles":   roles,
			})
			return

		default:
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 9410)
			return
		}
	}
}

// ========= /v1/admin/rbac/roles/{role}/perms (GET/POST) =========

type rbacRolePermsPayload struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

func AdminRBACRolePermsHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Path esperado: /v1/admin/rbac/roles/{role}/perms
		const pfx = "/v1/admin/rbac/roles/"
		if !strings.HasPrefix(r.URL.Path, pfx) || !strings.HasSuffix(r.URL.Path, "/perms") {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "ruta no encontrada", 9421)
			return
		}
		role := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, pfx), "/perms")
		role = strings.Trim(role, "/")
		if role == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_role", "role vacío", 9422)
			return
		}

		// Tomamos tenant_id del bearer (RequireSysAdmin ya garantizó auth)
		tenantID, err := parseBearerTenantID(c.Issuer, r)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "bearer inválido", 9423)
			return
		}

		rwr, okW := c.Store.(rbacWriteRepo)
		if !okW {
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta RBAC write", 9424)
			return
		}

		switch r.Method {
		case http.MethodGet:
			perms, err := rwr.GetRolePerms(r.Context(), tenantID, role)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9425)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"tenant_id": tenantID,
				"role":      role,
				"perms":     perms,
			})
			return

		case http.MethodPost:
			var p rbacRolePermsPayload
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "bad_json", "payload inválido", 9426)
				return
			}
			add := dedupTrim(p.Add)
			rm := dedupTrim(p.Remove)
			if len(add) > 0 {
				if err := rwr.AddRolePerms(r.Context(), tenantID, role, add); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9427)
					return
				}
			}
			if len(rm) > 0 {
				if err := rwr.RemoveRolePerms(r.Context(), tenantID, role, rm); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9428)
					return
				}
			}
			perms, err := rwr.GetRolePerms(r.Context(), tenantID, role)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9429)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"tenant_id": tenantID,
				"role":      role,
				"perms":     perms,
			})
			return

		default:
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 9430)
			return
		}
	}
}
