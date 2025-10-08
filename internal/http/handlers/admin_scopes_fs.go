package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
)

type adminScopesFS struct {
	container *app.Container
}

func NewAdminScopesFSHandler(c *app.Container) http.Handler {
	return &adminScopesFS{container: c}
}

func (h *adminScopesFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	base := "/v1/admin/scopes"
	path := r.URL.Path

	slug := "local"
	if v := r.Header.Get("X-Tenant-Slug"); v != "" {
		slug = v
	} else if v := r.URL.Query().Get("tenant"); v != "" {
		slug = v
	}

	write := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	writeErr := func(code int, msg string) {
		write(code, map[string]string{"error": msg})
	}

	switch {
	case path == base:
		switch r.Method {
		case http.MethodGet:
			scopes, err := cpctx.Provider.ListScopes(r.Context(), slug)
			if err != nil {
				writeErr(http.StatusInternalServerError, "list scopes failed")
				return
			}
			write(http.StatusOK, scopes)
			return

		case http.MethodPost, http.MethodPut:
			var s cp.Scope
			if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
				writeErr(http.StatusBadRequest, "invalid json")
				return
			}
			if err := cpctx.Provider.UpsertScope(r.Context(), slug, s); err != nil {
				writeErr(http.StatusBadRequest, "upsert failed: "+err.Error())
				return
			}
			write(http.StatusOK, s)
			return

		default:
			writeErr(http.StatusMethodNotAllowed, "method not allowed")
			return
		}

	case strings.HasPrefix(path, base+"/"):
		name := strings.TrimPrefix(path, base+"/")
		if name == "" {
			writeErr(http.StatusBadRequest, "missing scope name")
			return
		}
		switch r.Method {
		case http.MethodDelete:
			if err := cpctx.Provider.DeleteScope(r.Context(), slug, name); err != nil {
				writeErr(http.StatusBadRequest, "delete failed: "+err.Error())
				return
			}
			write(http.StatusOK, map[string]string{"status": "ok"})
			return
		default:
			writeErr(http.StatusMethodNotAllowed, "method not allowed")
			return
		}

	default:
		writeErr(http.StatusNotFound, "not found")
		return
	}
}
