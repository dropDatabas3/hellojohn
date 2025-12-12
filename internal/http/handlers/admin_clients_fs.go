package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/google/uuid"
)

type adminClientsFS struct {
	container *app.Container // reservado para logs/metrics si querés
}

func NewAdminClientsFSHandler(c *app.Container) http.Handler {
	return &adminClientsFS{container: c}
}

// isUUID checks if a string is a valid UUID
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// resolveTenantSlug converts a tenant ID (UUID) to its slug
func resolveTenantSlug(ctx context.Context, idOrSlug string) string {
	if !isUUID(idOrSlug) {
		return idOrSlug
	}
	if fsp, ok := cp.AsFSProvider(cpctx.Provider); ok {
		if t, err := fsp.GetTenantByID(ctx, idOrSlug); err == nil && t != nil {
			return t.Slug
		}
	}
	return idOrSlug // fallback
}

func (h *adminClientsFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Paths esperados:
	//   /v1/admin/clients               (GET=list, POST=create)
	//   /v1/admin/clients/{clientId}    (PUT=update, DELETE=delete)
	path := r.URL.Path
	base := "/v1/admin/clients"

	// Tenant slug: header > query. Accept both slug and id param names. Default "local".
	slug := "local"
	if v := r.Header.Get("X-Tenant-Slug"); v != "" {
		slug = v
	} else if v := r.Header.Get("X-Tenant-ID"); v != "" {
		slug = resolveTenantSlug(r.Context(), v)
	} else if v := r.URL.Query().Get("tenant"); v != "" {
		slug = v
	} else if v := r.URL.Query().Get("tenant_id"); v != "" {
		slug = resolveTenantSlug(r.Context(), v)
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
			if h.container != nil && h.container.ClusterNode != nil {
				dto := cluster.UpsertClientDTO{
					Name:                     in.Name,
					ClientID:                 in.ClientID,
					Type:                     in.Type,
					RedirectURIs:             in.RedirectURIs,
					AllowedOrigins:           in.AllowedOrigins,
					Providers:                in.Providers,
					Scopes:                   in.Scopes,
					Secret:                   in.Secret,
					RequireEmailVerification: in.RequireEmailVerification,
					ResetPasswordURL:         in.ResetPasswordURL,
					VerifyEmailURL:           in.VerifyEmailURL,
				}
				payload, _ := json.Marshal(dto)
				m := cluster.Mutation{
					Type:       cluster.MutationUpsertClient,
					TenantSlug: slug,
					TsUnix:     time.Now().Unix(),
					Payload:    payload,
				}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				// Read back and return client
				cobj, err := cpctx.Provider.GetClient(r.Context(), slug, in.ClientID)
				if err != nil {
					writeErr(http.StatusInternalServerError, "readback failed")
					return
				}
				write(http.StatusOK, cobj)
				return
			}
			// Fallback (cluster off): direct write
			cobj, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
			if err != nil {
				writeErr(http.StatusBadRequest, "create/update failed: "+err.Error())
				return
			}
			write(http.StatusOK, cobj)
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
			if h.container != nil && h.container.ClusterNode != nil {
				dto := cluster.UpsertClientDTO{
					Name:                     in.Name,
					ClientID:                 in.ClientID,
					Type:                     in.Type,
					RedirectURIs:             in.RedirectURIs,
					AllowedOrigins:           in.AllowedOrigins,
					Providers:                in.Providers,
					Scopes:                   in.Scopes,
					Secret:                   in.Secret,
					RequireEmailVerification: in.RequireEmailVerification,
					ResetPasswordURL:         in.ResetPasswordURL,
					VerifyEmailURL:           in.VerifyEmailURL,
				}
				payload, _ := json.Marshal(dto)
				m := cluster.Mutation{
					Type:       cluster.MutationUpsertClient,
					TenantSlug: slug,
					TsUnix:     time.Now().Unix(),
					Payload:    payload,
				}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				// read back
				cobj, err := cpctx.Provider.GetClient(r.Context(), slug, in.ClientID)
				if err != nil {
					writeErr(http.StatusInternalServerError, "readback failed")
					return
				}
				write(http.StatusOK, cobj)
				return
			}
			// fallback
			cobj, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
			if err != nil {
				writeErr(http.StatusBadRequest, "update failed: "+err.Error())
				return
			}
			write(http.StatusOK, cobj)
			return

		case http.MethodDelete:
			// If cluster is present, apply mutation; otherwise direct delete
			if h.container != nil && h.container.ClusterNode != nil {
				payload, _ := json.Marshal(cluster.DeleteClientDTO{ClientID: clientID})
				m := cluster.Mutation{Type: cluster.MutationDeleteClient, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				write(http.StatusOK, map[string]string{"status": "ok"})
				return
			}
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
