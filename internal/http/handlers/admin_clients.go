package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AdminClientsHandler struct{ c *app.Container }

func NewAdminClientsHandler(c *app.Container) *AdminClientsHandler { return &AdminClientsHandler{c: c} }

// helper: extrae el primer segmento después del prefijo
func pathID(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func (h *AdminClientsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c.Store == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store requerido", 3000)
		return
	}

	switch {
	// POST /v1/admin/clients  (create)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/clients":
		var body core.Client
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.TenantID = strings.TrimSpace(body.TenantID)
		body.ClientID = strings.TrimSpace(body.ClientID)
		body.Name = strings.TrimSpace(body.Name)
		body.ClientType = strings.TrimSpace(body.ClientType)

		if body.TenantID == "" || body.ClientID == "" || body.Name == "" || body.ClientType == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, name, client_type obligatorios", 3001)
			return
		}
		if err := h.c.Store.CreateClient(r.Context(), &body); err != nil {
			code := http.StatusBadRequest
			if err == core.ErrConflict {
				code = http.StatusConflict
			}
			httpx.WriteError(w, code, "create_failed", err.Error(), 3002)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(body)

	// GET /v1/admin/clients  (list)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/clients":
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenantID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant_id", "tenant_id requerido", 3011)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		items, err := h.c.Store.ListClients(r.Context(), tenantID, q)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "list_failed", err.Error(), 3012)
			return
		}
		_ = json.NewEncoder(w).Encode(items)

	// GET /v1/admin/clients/{id}
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/"):
		id := pathID(r.URL.Path, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3021)
			return
		}
		c, v, err := h.c.Store.GetClientByID(r.Context(), id)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusNotFound
			}
			httpx.WriteError(w, status, "get_failed", err.Error(), 3022)
			return
		}
		resp := map[string]any{"client": c, "active_version": v}
		_ = json.NewEncoder(w).Encode(resp)

	// PUT /v1/admin/clients/{id}
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/"):
		id := pathID(r.URL.Path, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3031)
			return
		}
		var body core.Client
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.ID = id
		if err := h.c.Store.UpdateClient(r.Context(), &body); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "update_failed", err.Error(), 3032)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	// DELETE /v1/admin/clients/{id}
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/"):
		id := pathID(r.URL.Path, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3041)
			return
		}
		soft := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("soft")), "true")
		if soft {
			_ = h.c.Store.RevokeAllRefreshTokensByClient(r.Context(), id)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = h.c.Store.RevokeAllRefreshTokensByClient(r.Context(), id)
		if err := h.c.Store.DeleteClient(r.Context(), id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "delete_failed", err.Error(), 3042)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	// POST /v1/admin/clients/{id}/revoke
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/") && strings.HasSuffix(r.URL.Path, "/revoke"):
		raw := strings.TrimPrefix(r.URL.Path, "/v1/admin/clients/")
		// por si aparece algo más después de /revoke (defensivo)
		if i := strings.Index(raw, "/revoke"); i >= 0 {
			raw = raw[:i]
		}
		id := pathID("/v1/admin/clients/"+raw, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3051)
			return
		}
		if err := h.c.Store.RevokeAllRefreshTokensByClient(r.Context(), id); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "revoke_failed", err.Error(), 3052)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.NotFound(w, r)
	}
}
