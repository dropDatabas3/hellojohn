package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/validation"
)

type AdminScopesHandler struct{ c *app.Container }

func NewAdminScopesHandler(c *app.Container) *AdminScopesHandler { return &AdminScopesHandler{c: c} }

func (h *AdminScopesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c.ScopesConsents == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "scopes/consents no soportado por este driver", 2400)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/scopes":
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenantID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant_id", "tenant_id requerido", 2401)
			return
		}
		items, err := h.c.ScopesConsents.ListScopes(r.Context(), tenantID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 2402)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, items)

	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/scopes":
		var body struct {
			TenantID    string `json:"tenant_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		tenantID := strings.TrimSpace(body.TenantID)
		rawName := strings.TrimSpace(body.Name)
		if tenantID == "" || rawName == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id y name requeridos", 2403)
			return
		}
		// Rechazar mayúsculas antes de mutar
		if rawName != strings.ToLower(rawName) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_scope_name", "el nombre debe estar en minúsculas", 2405)
			return
		}
		if !validation.ValidScopeName(rawName) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_scope_name", "formato inválido: usar [a-z0-9:_-.], 1–64, empieza/termina alfanumérico", 2405)
			return
		}
		name := strings.ToLower(rawName) // persistimos en minúsculas (idempotente)
		res, err := h.c.ScopesConsents.CreateScope(r.Context(), tenantID, name, body.Description)
		if err != nil {
			if errors.Is(err, core.ErrConflict) {
				httpx.WriteError(w, http.StatusConflict, "scope_exists", "ya existe un scope con ese nombre", 2406)
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "create_failed", err.Error(), 2404)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, res)

	// PUT /v1/admin/scopes/{id} - patch-like (solo description, sin exigir tenant_id)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/admin/scopes/"):
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/admin/scopes/"))
		if id == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "scope_id requerido", 2410)
			return
		}
		// Usamos punteros para distinguir campo ausente de string vacío.
		var body struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
		}
		if !httpx.ReadJSON(w, r, &body) { // ReadJSON ya maneja errores 400
			return
		}

		// Renombrar no soportado
		if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
			httpx.WriteError(w, http.StatusBadRequest, "rename_not_supported", "no se puede cambiar el nombre del scope", 2413)
			return
		}

		// Si no viene description (nil) -> no hay nada que actualizar: devolvemos 204 idempotente.
		if body.Description == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := h.c.ScopesConsents.UpdateScopeDescriptionByID(r.Context(), id, strings.TrimSpace(*body.Description)); err != nil {
			if errors.Is(err, core.ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "scope no encontrado", 2411)
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "update_failed", err.Error(), 2412)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/admin/scopes/"):
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/admin/scopes/"))
		if id == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "scope_id requerido", 2420)
			return
		}
		if err := h.c.ScopesConsents.DeleteScopeByID(r.Context(), id); err != nil {
			if errors.Is(err, core.ErrConflict) {
				httpx.WriteError(w, http.StatusConflict, "scope_in_use", "no se puede borrar: en uso", 2421)
				return
			}
			if errors.Is(err, core.ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "scope no encontrado", 2422)
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "delete_failed", err.Error(), 2423)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.NotFound(w, r)
	}
}
