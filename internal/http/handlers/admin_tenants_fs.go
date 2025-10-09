package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9\-]+$`)
)

// NewAdminTenantsFSHandler expone endpoints básicos para tenants:
//
//	GET    /v1/admin/tenants
//	POST   /v1/admin/tenants
//	GET    /v1/admin/tenants/{slug}
//	GET    /v1/admin/tenants/{slug}/settings
//	PUT    /v1/admin/tenants/{slug}/settings
func NewAdminTenantsFSHandler(c *app.Container) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		base := "/v1/admin/tenants"

		// Helper para generar ETag desde bytes
		etag := func(data []byte) string {
			h := sha256.Sum256(data)
			return `"` + hex.EncodeToString(h[:8]) + `"`
		}

		// Validar slug
		validSlug := func(slug string) bool {
			return len(slug) > 0 && len(slug) <= 32 && slugRegex.MatchString(slug)
		}

		// Obtener FSProvider
		fsProvider, ok := controlplane.AsFSProvider(cpctx.Provider)
		if !ok {
			httpx.WriteError(w, http.StatusNotImplemented, "fs_provider_required", "solo FS provider soportado", 5001)
			return
		}

		switch {
		case path == base:
			switch r.Method {
			case http.MethodGet:
				// Listar todos los tenants
				tenants, err := fsProvider.ListTenants(r.Context())
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "list_failed", err.Error(), 5002)
					return
				}
				httpx.WriteJSON(w, http.StatusOK, tenants)
				return

			case http.MethodPost:
				// Crear nuevo tenant
				var req struct {
					Name string `json:"name"`
					Slug string `json:"slug"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5003)
					return
				}

				req.Name = strings.TrimSpace(req.Name)
				req.Slug = strings.TrimSpace(req.Slug)

				if req.Name == "" || req.Slug == "" {
					httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "name y slug son requeridos", 5004)
					return
				}

				if !validSlug(req.Slug) {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", "slug debe ser [a-z0-9\\-] y <=32 chars", 5005)
					return
				}

				// Crear tenant usando UpsertTenant
				now := time.Now().UTC()
				tenant := &controlplane.Tenant{
					ID:        req.Slug, // Usar slug como ID por simplicidad
					Name:      req.Name,
					Slug:      req.Slug,
					CreatedAt: now,
					UpdatedAt: now,
					Settings:  controlplane.TenantSettings{}, // Settings vacías por defecto
				}

				if err := fsProvider.UpsertTenant(r.Context(), tenant); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "create_failed", err.Error(), 5006)
					return
				}

				httpx.WriteJSON(w, http.StatusCreated, tenant)
				return

			default:
				httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 5000)
				return
			}

		case strings.HasPrefix(path, base+"/"):
			// /v1/admin/tenants/{slug}[/settings]
			rest := strings.TrimPrefix(path, base+"/")
			parts := strings.Split(strings.Trim(rest, "/"), "/")
			if len(parts) == 0 || parts[0] == "" {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5010)
				return
			}

			slug := parts[0]

			// PUT /v1/admin/tenants/{slug} -> Upsert tenant (id, slug, settings)
			if len(parts) == 1 && r.Method == http.MethodPut {
				var in struct {
					ID       string                      `json:"id"`
					Slug     string                      `json:"slug"`
					Name     string                      `json:"name"`
					Settings controlplane.TenantSettings `json:"settings"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5030)
					return
				}
				if strings.TrimSpace(in.Slug) == "" {
					in.Slug = slug
				}
				if !validSlug(in.Slug) || in.Slug != slug {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", "slug inválido o mismatch con path", 5031)
					return
				}
				now := time.Now().UTC()
				t := &controlplane.Tenant{
					ID:        strings.TrimSpace(in.ID),
					Name:      strings.TrimSpace(in.Name),
					Slug:      in.Slug,
					CreatedAt: now,
					UpdatedAt: now,
					Settings:  in.Settings,
				}
				if strings.TrimSpace(t.ID) == "" {
					// fallback: usar slug como id en MVP
					t.ID = in.Slug
				}
				if err := fsProvider.UpsertTenant(r.Context(), t); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "upsert_failed", err.Error(), 5032)
					return
				}
				// Si llegaron settings con DSN en claro, UpsertTenant preserva settings; para cifrado fino usamos UpdateTenantSettings
				if (in.Settings != controlplane.TenantSettings{}) {
					if err := fsProvider.UpdateTenantSettings(r.Context(), slug, &in.Settings); err != nil {
						// no fatal; devolvemos 200 con warning implícito
					}
				}
				httpx.WriteJSON(w, http.StatusOK, t)
				return
			}

			// ─── Admin: per-tenant user-store ───
			// POST /v1/admin/tenants/{slug}/user-store/test-connection
			// POST /v1/admin/tenants/{slug}/user-store/migrate
			if len(parts) >= 2 && parts[1] == "user-store" {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "tenant_sql_manager_required", "TenantSQLManager no inicializado", 5025)
					return
				}

				if len(parts) == 3 && parts[2] == "test-connection" {
					if r.Method != http.MethodPost {
						httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5026)
						return
					}
					// First, verify tenant exists in FS; admin over nonexistent tenant -> 404
					if _, err := fsProvider.GetTenantBySlug(r.Context(), slug); err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}
					store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
					if err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}
					if pingErr := store.Ping(r.Context()); pingErr != nil {
						httpx.WriteTenantDBError(w, pingErr.Error())
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}

				if len(parts) == 3 && parts[2] == "migrate" {
					if r.Method != http.MethodPost {
						httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5027)
						return
					}
					// Verify tenant exists (404 if not)
					if _, err := fsProvider.GetTenantBySlug(r.Context(), slug); err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}
					// Nota: GetPG ya corre migraciones on-demand con lock.
					if _, err := c.TenantSQLManager.GetPG(r.Context(), slug); err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}

				httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5028)
				return
			}

			if len(parts) == 1 && r.Method == http.MethodGet {
				// GET /v1/admin/tenants/{slug}
				tenant, raw, err := fsProvider.GetTenantRaw(r.Context(), slug)
				if err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
					return
				}

				w.Header().Set("ETag", etag(raw))
				httpx.WriteJSON(w, http.StatusOK, tenant)
				return
			}

			if len(parts) == 2 && parts[1] == "settings" {
				switch r.Method {
				case http.MethodGet:
					// GET /v1/admin/tenants/{slug}/settings
					settings, raw, err := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5013)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5014)
						return
					}

					w.Header().Set("ETag", etag(raw))
					httpx.WriteJSON(w, http.StatusOK, settings)
					return

				case http.MethodPut:
					// PUT /v1/admin/tenants/{slug}/settings (con If-Match)
					ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
					if ifMatch == "" {
						httpx.WriteError(w, http.StatusPreconditionRequired, "if_match_required", "If-Match header requerido", 5015)
						return
					}

					// Verificar ETag actual
					_, raw, err := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5016)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5017)
						return
					}

					currentETag := etag(raw)
					if currentETag != ifMatch {
						httpx.WriteError(w, http.StatusPreconditionFailed, "etag_mismatch", "la versión cambió, recarga y reintenta", 5018)
						return
					}

					var newSettings controlplane.TenantSettings
					if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5019)
						return
					}

					// Actualizar settings (con cifrado automático)
					if err := fsProvider.UpdateTenantSettings(r.Context(), slug, &newSettings); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "update_failed", err.Error(), 5020)
						return
					}

					// Devolver nuevo ETag
					_, newRaw, _ := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
					w.Header().Set("ETag", etag(newRaw))
					httpx.WriteJSON(w, http.StatusOK, map[string]any{"updated": true})
					return

				default:
					httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/PUT", 5021)
					return
				}
			}

			// Under tenant: clients CRUD via FS
			if len(parts) >= 2 && parts[1] == "clients" {
				// PUT /v1/admin/tenants/{slug}/clients/{clientID}
				if len(parts) == 3 && r.Method == http.MethodPut {
					var in controlplane.ClientInput
					if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5040)
						return
					}
					in.ClientID = parts[2]
					c, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
					if err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "upsert_failed", err.Error(), 5041)
						return
					}
					httpx.WriteJSON(w, http.StatusOK, c)
					return
				}
			}

			// Under tenant: scopes bulk upsert (simple array)
			if len(parts) == 2 && parts[1] == "scopes" && (r.Method == http.MethodPut || r.Method == http.MethodPost) {
				var in struct {
					Scopes []controlplane.Scope `json:"scopes"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5050)
					return
				}
				// naive: replace one by one using UpsertScope
				for _, s := range in.Scopes {
					_ = cpctx.Provider.UpsertScope(r.Context(), slug, controlplane.Scope{Name: strings.TrimSpace(s.Name), Description: s.Description})
				}
				httpx.WriteJSON(w, http.StatusOK, map[string]any{"updated": true})
				return
			}

			httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5010)
			return

		default:
			httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5010)
			return
		}
	})
}
