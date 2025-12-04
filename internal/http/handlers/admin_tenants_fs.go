package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantcache"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/google/uuid"
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
					Name     string                      `json:"name"`
					Slug     string                      `json:"slug"`
					Settings controlplane.TenantSettings `json:"settings"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5003)
					return
				}

				// Leader enforcement moved to middleware

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

				// Encrypt sensitive fields
				masterKey := os.Getenv("SIGNING_MASTER_KEY")
				if masterKey != "" {
					// SMTP Password
					if req.Settings.SMTP != nil && req.Settings.SMTP.Password != "" {
						if enc, err := jwtx.EncryptPrivateKey([]byte(req.Settings.SMTP.Password), masterKey); err == nil {
							req.Settings.SMTP.PasswordEnc = jwtx.EncodeBase64URL(enc)
							req.Settings.SMTP.Password = "" // Clear plain
						}
					}
					// UserDB DSN
					if req.Settings.UserDB != nil && req.Settings.UserDB.DSN != "" {
						if enc, err := jwtx.EncryptPrivateKey([]byte(req.Settings.UserDB.DSN), masterKey); err == nil {
							req.Settings.UserDB.DSNEnc = jwtx.EncodeBase64URL(enc)
							req.Settings.UserDB.DSN = "" // Clear plain
						}
					}
					// Cache Password
					if req.Settings.Cache != nil && req.Settings.Cache.Password != "" {
						if enc, err := jwtx.EncryptPrivateKey([]byte(req.Settings.Cache.Password), masterKey); err == nil {
							req.Settings.Cache.PassEnc = jwtx.EncodeBase64URL(enc)
							req.Settings.Cache.Password = "" // Clear plain
						}
					}
				}

				// Crear tenant via Raft si existe cluster, sino directo
				if c != nil && c.ClusterNode != nil {
					payload, _ := json.Marshal(cluster.UpsertTenantDTO{ID: req.Slug, Name: req.Name, Slug: req.Slug, Settings: req.Settings})
					m := cluster.Mutation{Type: cluster.MutationUpsertTenant, TenantSlug: req.Slug, TsUnix: time.Now().Unix(), Payload: payload}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					// read back
					if t, err := fsProvider.GetTenantBySlug(r.Context(), req.Slug); err == nil {
						httpx.WriteJSON(w, http.StatusCreated, t)
						return
					}
				}
				now := time.Now().UTC()
				tenant := &controlplane.Tenant{ID: req.Slug, Name: req.Name, Slug: req.Slug, CreatedAt: now, UpdatedAt: now, Settings: req.Settings}
				if err := fsProvider.UpsertTenant(r.Context(), tenant); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "create_failed", err.Error(), 5006)
					return
				}

				// Generate initial keys for the new tenant
				if c != nil && c.Issuer != nil {
					if _, err := c.Issuer.Keys.RotateFor(req.Slug, 0); err != nil {
						// Log warning but don't fail the request as tenant is created
						// Keys will be generated on first use or next rotation
						fmt.Printf("WARN: failed to generate initial keys for tenant %s: %v\n", req.Slug, err)
					}
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

			// Support lookup by ID: if slug looks like UUID, try to resolve it
			if _, err := uuid.Parse(slug); err == nil {
				if t, err := fsProvider.GetTenantByID(r.Context(), slug); err == nil {
					slug = t.Slug
				}
				// If not found by ID, we continue assuming it might be a weird slug (unlikely but safe)
				// or it will fail later with "tenant not found"
			}

			// POST /v1/admin/tenants/{slug}/keys/rotate
			if len(parts) == 3 && parts[1] == "keys" && parts[2] == "rotate" {
				if r.Method != http.MethodPost {
					httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5060)
					return
				}
				// Verificar tenant existe (404 si no)
				if _, err := fsProvider.GetTenantBySlug(r.Context(), slug); err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
					return
				}

				// Leer graceSeconds: query param tiene prioridad, luego ENV KEY_ROTATION_GRACE_SECONDS, default 60
				var grace int64 = 60
				if env := strings.TrimSpace(os.Getenv("KEY_ROTATION_GRACE_SECONDS")); env != "" {
					if v, err := strconv.ParseInt(env, 10, 64); err == nil && v >= 0 {
						grace = v
					}
				}
				if qs := r.URL.Query().Get("graceSeconds"); qs != "" {
					if v, err := strconv.ParseInt(qs, 10, 64); err == nil && v >= 0 {
						grace = v
					}
				}

				// Rotación replicada: en líder, rotar e incluir material exacto para seguidores
				if c != nil && c.ClusterNode != nil {
					// Perform rotation on leader using keystore, then read active/retiring JSON files to include in mutation
					sk, err := c.Issuer.Keys.RotateFor(slug, grace)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 5061)
						return
					}
					// Resolve FS root via provider
					fsProv, ok := controlplane.AsFSProvider(cpctx.Provider)
					if !ok {
						httpx.WriteError(w, http.StatusInternalServerError, "server_error", "fs provider required for rotation", 5062)
						return
					}
					keysDir := filepath.Join(fsProv.FSRoot(), "keys", slug)
					// Read file contents as-is
					actBytes, aerr := os.ReadFile(filepath.Join(keysDir, "active.json"))
					if aerr != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "server_error", aerr.Error(), 5063)
						return
					}
					var retBytes []byte
					if b, rerr := os.ReadFile(filepath.Join(keysDir, "retiring.json")); rerr == nil {
						retBytes = b
					}
					dto := cluster.RotateTenantKeyDTO{
						ActiveJSON:   string(actBytes),
						RetiringJSON: string(retBytes),
						GraceSeconds: grace,
					}
					payload, _ := json.Marshal(dto)
					m := cluster.Mutation{Type: cluster.MutationRotateTenantKey, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					// Invalidate JWKS locally (followers will do so during Apply)
					if c.JWKSCache != nil {
						c.JWKSCache.Invalidate(slug)
					}
					setNoStore(w)
					httpx.WriteJSON(w, http.StatusOK, map[string]any{"kid": sk.KID})
					return
				}
				// Fallback (no cluster): local rotate only
				if sk, err := c.Issuer.Keys.RotateFor(slug, grace); err == nil {
					if c.JWKSCache != nil {
						c.JWKSCache.Invalidate(slug)
					}
					setNoStore(w)
					httpx.WriteJSON(w, http.StatusOK, map[string]any{"kid": sk.KID})
					return
				} else {
					httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 5061)
					return
				}
			}

			// PUT /v1/admin/tenants/{slug} -> Upsert tenant (id, slug, settings)
			if len(parts) == 1 && r.Method == http.MethodPut {
				var in struct {
					ID          string                      `json:"id"`
					Slug        string                      `json:"slug"`
					Name        string                      `json:"name"`
					DisplayName string                      `json:"display_name"`
					Settings    controlplane.TenantSettings `json:"settings"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5030)
					return
				}
				// Leader enforcement moved to middleware
				if strings.TrimSpace(in.Slug) == "" {
					in.Slug = slug
				}
				if !validSlug(in.Slug) || in.Slug != slug {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", "slug inválido o mismatch con path", 5031)
					return
				}
				// Validar issuerMode (permite vacío como "global").
				if err := validateIssuerMode(in.Settings.IssuerMode); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_issuer_mode", err.Error(), 5033)
					return
				}
				if c != nil && c.ClusterNode != nil {
					payload, _ := json.Marshal(cluster.UpsertTenantDTO{ID: strings.TrimSpace(in.ID), Name: strings.TrimSpace(in.Name), Slug: in.Slug, Settings: in.Settings})
					m := cluster.Mutation{Type: cluster.MutationUpsertTenant, TenantSlug: in.Slug, TsUnix: time.Now().Unix(), Payload: payload}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					if t, err := fsProvider.GetTenantBySlug(r.Context(), in.Slug); err == nil {
						httpx.WriteJSON(w, http.StatusOK, t)
						return
					}
				}
				now := time.Now().UTC()
				t := &controlplane.Tenant{
					ID:          strings.TrimSpace(in.ID),
					Name:        strings.TrimSpace(in.Name),
					DisplayName: strings.TrimSpace(in.DisplayName),
					Slug:        in.Slug,
					CreatedAt:   now,
					UpdatedAt:   now,
					Settings:    in.Settings,
				}
				if strings.TrimSpace(t.ID) == "" {
					// Try to preserve existing ID
					if existing, err := fsProvider.GetTenantBySlug(r.Context(), in.Slug); err == nil {
						t.ID = existing.ID
						t.CreatedAt = existing.CreatedAt // Preserve creation time too
					} else {
						// If new, leave empty to let ensureTenantID generate UUID
						// t.ID = in.Slug // REMOVED: Do not default to slug as it triggers regeneration if not UUID
					}
				}
				// Update settings via provider
				if err := fsProvider.UpdateTenantSettings(r.Context(), slug, &in.Settings); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "update_failed", err.Error(), 5032)
					return
				}

				// FIX: La comparación anterior fallaba por slices.
				// Asumimos que si llegamos aquí es un upsert completo, así que settings se actualizan.
				// Además, si hay UserFields (incluso vacío si se borraron todos), sincronizamos el esquema.
				if in.Settings.UserFields != nil {
					// Obtener store para sincronizar
					if c.TenantSQLManager != nil {
						if store, err := c.TenantSQLManager.GetPG(r.Context(), slug); err == nil {
							// Usar schema manager
							sm := tenantsql.NewSchemaManager(store.Pool())
							// Obtener tenantID real si es posible, sino slug
							tid := t.ID
							if tid == "" {
								tid = slug
							}

							// Ejecutar bajo lock de migración para evitar conflictos
							_ = tenantsql.WithTenantMigrationLock(r.Context(), store.Pool(), tid, 10*time.Second, func(ctx context.Context) error {
								return sm.SyncUserFields(ctx, tid, in.Settings.UserFields)
							})
						}
					}
				}

				httpx.WriteJSON(w, http.StatusOK, t)
				return
			}

			// DELETE /v1/admin/tenants/{slug} -> Delete tenant
			if len(parts) == 1 && r.Method == http.MethodDelete {
				// Leader enforcement moved to middleware
				if c != nil && c.ClusterNode != nil {
					m := cluster.Mutation{Type: cluster.MutationDeleteTenant, TenantSlug: slug, TsUnix: time.Now().Unix()}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if err := fsProvider.DeleteTenant(r.Context(), slug); err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "delete_failed", err.Error(), 5035)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// ─── Admin: per-tenant users ───
			// GET  /v1/admin/tenants/{slug}/users
			// POST /v1/admin/tenants/{slug}/users
			if len(parts) == 2 && parts[1] == "users" {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5080)
					return
				}
				// Obtener store del tenant
				store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
				if err != nil {
					if tenantsql.IsNoDBForTenant(err) {
						httpx.WriteTenantDBMissing(w)
						return
					}
					httpx.WriteTenantDBError(w, err.Error())
					return
				}

				// GET: Listar usuarios
				if r.Method == http.MethodGet {
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "get_tenant_failed", err.Error(), 5081)
						return
					}

					// Usar interface casting para acceder a ListUsers
					type userLister interface {
						ListUsers(ctx context.Context, tenantID string) ([]core.User, error)
					}

					if lister, ok := any(store).(userLister); ok {
						users, err := lister.ListUsers(r.Context(), t.ID)
						if err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "list_users_failed", err.Error(), 5082)
							return
						}
						httpx.WriteJSON(w, http.StatusOK, users)
						return
					}
					httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store no soporta listar usuarios", 5083)
					return
				}

				// POST: Crear usuario
				if r.Method == http.MethodPost {
					var req struct {
						Email         string         `json:"email"`
						Password      string         `json:"password"`
						EmailVerified bool           `json:"email_verified"`
						CustomFields  map[string]any `json:"custom_fields"`
					}
					if !httpx.ReadJSON(w, r, &req) {
						return
					}
					req.Email = strings.TrimSpace(strings.ToLower(req.Email))
					if req.Email == "" || req.Password == "" {
						httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "email y password requeridos", 5084)
						return
					}

					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "get_tenant_failed", err.Error(), 5081)
						return
					}

					phc, err := password.Hash(password.Default, req.Password)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "hash_failed", "no se pudo hashear el password", 5085)
						return
					}

					u := &core.User{
						TenantID:      t.ID,
						Email:         req.Email,
						EmailVerified: req.EmailVerified,
						Metadata:      map[string]any{},
						CustomFields:  req.CustomFields,
					}

					if err := store.CreateUser(r.Context(), u); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "create_user_failed", err.Error(), 5086)
						return
					}

					// Crear identidad password
					if err := store.CreatePasswordIdentity(r.Context(), u.ID, req.Email, req.EmailVerified, phc); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "create_identity_failed", err.Error(), 5087)
						return
					}

					httpx.WriteJSON(w, http.StatusCreated, u)
					return
				}

				httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 5088)
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
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
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
					// Ejecutar ping bajo el lock (misma conexión)
					if err := tenantsql.WithTenantMigrationLock(r.Context(), store.Pool(), t.ID, 5*time.Second, func(ctx context.Context) error {
						return store.Ping(ctx)
					}); err != nil {
						httpx.WriteTenantDBError(w, err.Error())
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
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}
					// Short-circuit: if no pending migrations, return 204 immediately
					if c.TenantSQLManager != nil {
						if has, herr := c.TenantSQLManager.HasPendingMigrations(r.Context(), t.ID); herr == nil && !has {
							w.WriteHeader(http.StatusNoContent)
							return
						}
					}
					// Usar MigrateTenant del manager que ya tiene configurado el path correcto y maneja el lock
					count, err := c.TenantSQLManager.MigrateTenant(r.Context(), slug)
					if err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						// Si fue timeout/contención, devolver 409 con Retry-After
						if errors.Is(err, context.DeadlineExceeded) {
							w.Header().Set("Retry-After", "5")
							httpx.WriteError(w, http.StatusConflict, "conflict", "migration lock busy, retry later", 2605)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}

					// Log success (optional, manager already logs)
					_ = count
					w.WriteHeader(http.StatusNoContent)
					return
				}

				httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5028)
				return
			}

			// ─── Admin: per-tenant cache ───
			// POST /v1/admin/tenants/{slug}/cache/test-connection
			if len(parts) >= 2 && parts[1] == "cache" {
				if c.TenantCacheManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "tenant_cache_manager_required", "TenantCacheManager no inicializado", 5070)
					return
				}

				if len(parts) == 3 && parts[2] == "test-connection" {
					if r.Method != http.MethodPost {
						httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5071)
						return
					}
					// Verify tenant exists
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}

					// Get cache client (creates connection if needed)
					client, err := c.TenantCacheManager.Get(r.Context(), t.Slug)
					if err != nil {
						if err == tenantcache.ErrNoCacheForTenant {
							// If no cache configured, we can consider it "success" if using default memory,
							// or error if explicit test requested. For now, let's return 404-like or specific error.
							// Better: return 400 saying no cache config.
							httpx.WriteError(w, http.StatusBadRequest, "no_cache_config", "no cache configured for tenant", 5072)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "cache_error", err.Error(), 5073)
						return
					}

					// Ping
					if err := client.Ping(r.Context()); err != nil {
						httpx.WriteError(w, http.StatusBadGateway, "cache_ping_failed", err.Error(), 5074)
						return
					}

					w.WriteHeader(http.StatusNoContent)
					return
				}

				httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5028)
				return
			}

			// GET /v1/admin/tenants/{slug}/infra-stats
			if len(parts) == 2 && parts[1] == "infra-stats" && r.Method == http.MethodGet {
				stats := make(map[string]any)

				// DB Stats
				if c.TenantSQLManager != nil {
					if dbStats, err := c.TenantSQLManager.GetStats(r.Context(), slug); err == nil {
						stats["db"] = dbStats
					} else {
						stats["db_error"] = err.Error()
					}
				}

				// Cache Stats
				if c.TenantCacheManager != nil {
					if client, err := c.TenantCacheManager.Get(r.Context(), slug); err == nil {
						if cacheStats, err := client.Stats(r.Context()); err == nil {
							stats["cache"] = cacheStats
						} else {
							stats["cache_error"] = err.Error()
						}
					} else {
						// Ignore error if just not configured, but report if other error
						if err != tenantcache.ErrNoCacheForTenant {
							stats["cache_error"] = err.Error()
						}
					}
				}

				httpx.WriteJSON(w, http.StatusOK, stats)
				return
			}

			// POST /v1/admin/tenants/{slug}/migrate
			if len(parts) == 2 && parts[1] == "migrate" && r.Method == http.MethodPost {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5075)
					return
				}

				count, err := c.TenantSQLManager.MigrateTenant(r.Context(), slug)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "migrate_failed", err.Error(), 5076)
					return
				}

				httpx.WriteJSON(w, http.StatusOK, map[string]any{
					"status":  "success",
					"applied": count,
				})
				return
			}

			// POST /v1/admin/tenants/{slug}/schema/apply
			if len(parts) == 3 && parts[1] == "schema" && parts[2] == "apply" && r.Method == http.MethodPost {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5077)
					return
				}

				// Parse schema definition from body
				var schemaDef map[string]any
				if err := json.NewDecoder(r.Body).Decode(&schemaDef); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body", 5078)
					return
				}

				// Get store to reuse connection pool
				store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "db_connection_failed", err.Error(), 5079)
					return
				}

				// Initialize Schema Manager (Postgres implementation)
				// In a real multi-driver scenario, we would get the correct manager from a factory.
				schemaMgr := tenantsql.NewSchemaManager(store.Pool())

				// Apply Indexes
				if err := schemaMgr.EnsureIndexes(r.Context(), slug, schemaDef); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "schema_apply_failed", err.Error(), 5080)
					return
				}

				httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "applied"})
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
					// Leader enforcement moved to middleware
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

					// Encrypt sensitive fields
					masterKey := os.Getenv("SIGNING_MASTER_KEY")
					if masterKey != "" {
						// SMTP Password
						if newSettings.SMTP != nil && newSettings.SMTP.Password != "" {
							if enc, err := jwtx.EncryptPrivateKey([]byte(newSettings.SMTP.Password), masterKey); err == nil {
								newSettings.SMTP.PasswordEnc = jwtx.EncodeBase64URL(enc)
								newSettings.SMTP.Password = "" // Clear plain
							}
						}
						// UserDB DSN
						if newSettings.UserDB != nil && newSettings.UserDB.DSN != "" {
							if enc, err := jwtx.EncryptPrivateKey([]byte(newSettings.UserDB.DSN), masterKey); err == nil {
								newSettings.UserDB.DSNEnc = jwtx.EncodeBase64URL(enc)
								newSettings.UserDB.DSN = "" // Clear plain
							}
						}
						// Cache Password
						if newSettings.Cache != nil && newSettings.Cache.Password != "" {
							if enc, err := jwtx.EncryptPrivateKey([]byte(newSettings.Cache.Password), masterKey); err == nil {
								newSettings.Cache.PassEnc = jwtx.EncodeBase64URL(enc)
								newSettings.Cache.Password = "" // Clear plain
							}
						}
					}

					if err := validateIssuerMode(newSettings.IssuerMode); err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_issuer_mode", err.Error(), 5033)
						return
					}

					// Apply via Raft if cluster
					if c != nil && c.ClusterNode != nil {
						payload, _ := json.Marshal(cluster.UpdateTenantSettingsDTO{Settings: newSettings})
						m := cluster.Mutation{Type: cluster.MutationUpdateTenantSettings, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
						if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
							httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
							return
						}
						_, newRaw, _ := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
						w.Header().Set("ETag", etag(newRaw))
						httpx.WriteJSON(w, http.StatusOK, map[string]any{"updated": true})
						return
					}

					if err := fsProvider.UpdateTenantSettings(r.Context(), slug, &newSettings); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "update_failed", err.Error(), 5032)
						return
					}

					// Sync User Fields if present
					if newSettings.UserFields != nil {
						if c.TenantSQLManager != nil {
							if store, err := c.TenantSQLManager.GetPG(r.Context(), slug); err == nil {
								sm := tenantsql.NewSchemaManager(store.Pool())
								// Need tenant ID for lock, try to get it from context or slug
								// For now use slug as ID for lock if ID not handy, but better to fetch tenant.
								// We already fetched raw settings, maybe we can fetch tenant quickly or just use slug.
								// Using slug is safe enough for lock ID usually.
								_ = tenantsql.WithTenantMigrationLock(r.Context(), store.Pool(), slug, 10*time.Second, func(ctx context.Context) error {
									return sm.SyncUserFields(ctx, slug, newSettings.UserFields)
								})
							}
						}
					}

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
					// Leader enforcement moved to middleware
					if c != nil && c.ClusterNode != nil {
						dto := cluster.UpsertClientDTO{Name: in.Name, ClientID: in.ClientID, Type: in.Type, RedirectURIs: in.RedirectURIs, AllowedOrigins: in.AllowedOrigins, Providers: in.Providers, Scopes: in.Scopes, Secret: in.Secret}
						payload, _ := json.Marshal(dto)
						m := cluster.Mutation{Type: cluster.MutationUpsertClient, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
						if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
							httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
							return
						}
						co, err := cpctx.Provider.GetClient(r.Context(), slug, in.ClientID)
						if err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "readback_failed", err.Error(), 5042)
							return
						}
						httpx.WriteJSON(w, http.StatusOK, co)
						return
					}
					cobj, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
					if err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "upsert_failed", err.Error(), 5041)
						return
					}
					httpx.WriteJSON(w, http.StatusOK, cobj)
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

// validateIssuerMode valida el enum aceptando vacío como "global".
func validateIssuerMode(m controlplane.IssuerMode) error {
	switch m {
	case "", controlplane.IssuerModeGlobal, controlplane.IssuerModePath, controlplane.IssuerModeDomain:
		return nil
	default:
		return fmt.Errorf("invalid issuerMode: %q", m)
	}
}
