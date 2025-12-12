package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CompleteProfileRequest is the request body for completing user profile
type CompleteProfileRequest struct {
	CustomFields map[string]string `json:"custom_fields"`
}

// NewCompleteProfileHandler creates a handler for POST /v1/auth/complete-profile
// This endpoint allows authenticated users to update their custom fields.
func NewCompleteProfileHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}

		// 1. Validate Bearer token
		ah := strings.TrimSpace(r.Header.Get("Authorization"))
		if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authorization header requerido", 1850)
			return
		}
		raw := strings.TrimSpace(ah[len("Bearer "):])

		// Parse token to extract claims
		tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(), jwtv5.WithValidMethods([]string{"EdDSA"}))
		if err != nil || !tk.Valid {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "token inválido o expirado", 1851)
			return
		}
		claims, ok := tk.Claims.(jwtv5.MapClaims)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "claims inválidos", 1852)
			return
		}

		sub, _ := claims["sub"].(string)
		tid, _ := claims["tid"].(string)
		if sub == "" || tid == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_token", "sub/tid faltante en token", 1853)
			return
		}

		// 2. Read request body
		var req CompleteProfileRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		if len(req.CustomFields) == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "custom_fields vacíos", 1854)
			return
		}

		// 3. Resolve tenant slug from UUID
		tenantSlug := tid
		if cpctx.Provider != nil {
			if tenants, err := cpctx.Provider.ListTenants(r.Context()); err == nil {
				for _, t := range tenants {
					if t.ID == tid {
						tenantSlug = t.Slug
						break
					}
				}
			}
		}

		// 4. Get tenant store
		var userStore = c.Store // Default
		if c.TenantSQLManager != nil {
			if tStore, err := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); err == nil && tStore != nil {
				userStore = tStore
			}
		}

		// 5. Update user custom fields
		// First get current user to merge custom fields
		user, err := userStore.GetUserByID(r.Context(), sub)
		if err != nil || user == nil {
			httpx.WriteError(w, http.StatusNotFound, "user_not_found", "usuario no encontrado", 1855)
			return
		}

		// Merge new fields into existing metadata/custom_fields
		if user.Metadata == nil {
			user.Metadata = map[string]any{}
		}
		customFields, ok := user.Metadata["custom_fields"].(map[string]any)
		if !ok {
			customFields = map[string]any{}
		}
		for k, v := range req.CustomFields {
			customFields[k] = v
		}
		user.Metadata["custom_fields"] = customFields

		// 6. Save updated user using dynamic SQL to support real columns
		type poolGetter interface {
			Pool() *pgxpool.Pool
		}
		pg, ok := userStore.(poolGetter)
		if !ok {
			httpx.WriteError(w, http.StatusInternalServerError, "store_incompatible", "el store no soporta actualizaciones directas", 1857)
			return
		}
		pool := pg.Pool()

		// Introspect columns to separate real columns from metadata
		rows, err := pool.Query(r.Context(), `SELECT column_name FROM information_schema.columns WHERE table_name = 'app_user'`)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "introspect_failed", "error leyendo esquema", 1858)
			return
		}
		defer rows.Close()

		realColumns := make(map[string]bool)
		for rows.Next() {
			var colName string
			if err := rows.Scan(&colName); err == nil {
				realColumns[colName] = true
			}
		}

		// Prepare Update Query
		var setParts []string
		var args []any
		argIdx := 1

		// Fields to go into metadata
		metaUpdates := make(map[string]any)

		// Check each request field
		for k, v := range req.CustomFields {
			// Normalize key to lower case for column matching check? Postgres columns usually lowercase.
			// Let's try exact match or lower match.
			kLower := strings.ToLower(k)
			if realColumns[k] || realColumns[kLower] {
				// It's a real column
				col := k
				if realColumns[kLower] && !realColumns[k] {
					col = kLower
				}
				// FIX: Quote column name to handle spaces like "Pais de origen"
				setParts = append(setParts, fmt.Sprintf("\"%s\" = $%d", col, argIdx))
				args = append(args, v)
				argIdx++
			} else {
				// It goes to metadata
				metaUpdates[k] = v
			}
		}

		// Always merge metadata updates
		if len(metaUpdates) > 0 {
			// Fetch current metadata to merge
			currentMeta := user.Metadata
			if currentMeta == nil {
				currentMeta = make(map[string]any)
			}
			customFields, _ := currentMeta["custom_fields"].(map[string]any)
			if customFields == nil {
				customFields = make(map[string]any)
			}

			for k, v := range metaUpdates {
				customFields[k] = v
			}
			currentMeta["custom_fields"] = customFields

			setParts = append(setParts, fmt.Sprintf("metadata = $%d", argIdx))
			args = append(args, currentMeta)
			argIdx++
		}

		if len(setParts) == 0 {
			// Nothing to update?
			httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "no changes"})
			return
		}

		// Add WHERE clause
		q := fmt.Sprintf("UPDATE app_user SET %s WHERE id = $%d", strings.Join(setParts, ", "), argIdx)
		args = append(args, sub)

		_, err = pool.Exec(r.Context(), q, args...)
		if err != nil {
			log.Printf("CompleteProfile Update Error: %v | Query: %s", err, q)
			httpx.WriteError(w, http.StatusInternalServerError, "update_failed", "error actualizando perfil: "+err.Error(), 1856)
			return
		}

		// 7. Return success
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "Perfil actualizado correctamente",
		})
	}
}

