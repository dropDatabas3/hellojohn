package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
)

type adminClientsFS struct {
	container *app.Container // reservado para logs/metrics si querés
}

func NewAdminClientsFSHandler(c *app.Container) http.Handler {
	return &adminClientsFS{container: c}
}

func (h *adminClientsFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Paths esperados:
	//   /v1/admin/clients               (GET=list, POST=create)
	//   /v1/admin/clients/{clientId}    (PUT=update, DELETE=delete)
	path := r.URL.Path
	base := "/v1/admin/clients"

	// Tenant slug: header > query > "local"
	slug := "local"
	if v := r.Header.Get("X-Tenant-Slug"); v != "" {
		slug = v
	} else if v := r.URL.Query().Get("tenant"); v != "" {
		slug = v
	}

	// helper JSON
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
			clients, err := cpctx.Provider.ListClients(r.Context(), slug)
			if err != nil {
				writeErr(http.StatusInternalServerError, "list clients failed")
				return
			}
			write(http.StatusOK, clients)
			return

		case http.MethodPost:
			var in cp.ClientInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeErr(http.StatusBadRequest, "invalid json")
				return
			}
			c, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
			if err != nil {
				writeErr(http.StatusBadRequest, "create/update failed: "+err.Error())
				return
			}
			write(http.StatusOK, c)
			return

		default:
			writeErr(http.StatusMethodNotAllowed, "method not allowed")
			return
		}

	case strings.HasPrefix(path, base+"/"):
		clientID := strings.TrimPrefix(path, base+"/")
		if clientID == "" {
			writeErr(http.StatusBadRequest, "missing client id")
			return
		}

		switch r.Method {
		case http.MethodPut, http.MethodPatch:
			var in cp.ClientInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeErr(http.StatusBadRequest, "invalid json")
				return
			}
			// forzamos el clientId del path si no vino en body (o lo pisamos)
			in.ClientID = clientID
			c, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
			if err != nil {
				writeErr(http.StatusBadRequest, "update failed: "+err.Error())
				return
			}
			write(http.StatusOK, c)
			return

		case http.MethodDelete:
			if err := cpctx.Provider.DeleteClient(r.Context(), slug, clientID); err != nil {
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
