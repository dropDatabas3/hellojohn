package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

// compute at_hash = base64url( left-most 128 bits of SHA-256(access_token) )
func atHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:len(sum)/2]) // 16 bytes
}

func NewOAuthTokenHandler(c *app.Container, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Timeout de 3s para endpoint crítico
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}

		// Resolver store con precedencia: tenantDB > globalDB
		var activeStore core.Repository
		var hasStore bool

		if c.TenantSQLManager != nil {
			// Intentar obtener store del tenant actual
			tenantSlug := cpctx.ResolveTenant(r)
			if tenantStore, err := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); err == nil && tenantStore != nil {
				activeStore = tenantStore
				hasStore = true
			}
		}
		// Fallback a global store si no hay tenant store
		if !hasStore && c.Store != nil {
			activeStore = c.Store
			hasStore = true
		}
		// OAuth2: application/x-www-form-urlencoded
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64KB
		if err := r.ParseForm(); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "form inválido", 2201)
			return
		}
		grantType := strings.TrimSpace(r.PostForm.Get("grant_type"))

		switch grantType {

		// ───────────────── authorization_code + PKCE ─────────────────
		case "authorization_code":
			code := strings.TrimSpace(r.PostForm.Get("code"))
			redirectURI := strings.TrimSpace(r.PostForm.Get("redirect_uri"))
			clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
			codeVerifier := strings.TrimSpace(r.PostForm.Get("code_verifier"))

			if code == "" || redirectURI == "" || clientID == "" || codeVerifier == "" {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "faltan parámetros", 2203)
				return
			}

			ctx := r.Context()

			client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client not found", 2204)
				return
			}

			// TODO: Implementar ValidateClientSecret cuando se agregue auth del cliente
			// if err := helpers.ValidateClientSecret(ctx, r, tenantSlug, client, clientSecret); err != nil {
			//     httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "bad credentials", 2205)
			//     return
			// }

			// Mapear client FS a estructura legacy para compatibilidad
			cl := &core.Client{
				ID:           client.ClientID,
				TenantID:     tenantSlug,
				RedirectURIs: client.RedirectURIs,
				Scopes:       client.Scopes,
			}
			_ = tenantSlug

			// Cargar y consumir el code (1 uso)
			key := "oidc:code:" + tokens.SHA256Base64URL(code)
			data, ok := c.Cache.Get(key)
			if !ok {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "authorization code inválido", 2205)
				return
			}
			c.Cache.Delete(key)

			var ac authCode
			if err := json.Unmarshal(data, &ac); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "authorization code corrupto", 2206)
				return
			}
			// Expirado
			if time.Now().After(ac.ExpiresAt) {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "authorization code expirado", 2207)
				return
			}
			// Validar contra el UUID interno del client (ac.ClientID contiene cl.ID desde authorize)
			// Aceptamos que en el form venga el client_id "público": lo resolvemos y comparamos con cl.ID
			if ac.ClientID != cl.ID || ac.RedirectURI != redirectURI {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "client/redirect_uri no coinciden", 2208)
				return
			}
			// PKCE S256
			verifierHash := tokens.SHA256Base64URL(codeVerifier)
			if !strings.EqualFold(ac.ChallengeMethod, "S256") || !strings.EqualFold(ac.CodeChallenge, verifierHash) {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "PKCE inválido", 2209)
				return
			}

			// Access Token (std/custom + hook)
			reqScopes := strings.Fields(ac.Scope)
			accessAMR := ac.AMR
			acrVal := "urn:hellojohn:loa:1"
			for _, v := range accessAMR {
				if v == "mfa" {
					acrVal = "urn:hellojohn:loa:2"
					break
				}
			}
			std := map[string]any{
				"tid":   ac.TenantID,
				"amr":   accessAMR,
				"acr":   acrVal,
				"scope": strings.Join(reqScopes, " "),
				"scp":   reqScopes, // compatibilidad con SDKs que esperan lista
			}
			custom := map[string]any{}

			std, custom = applyAccessClaimsHook(ctx, c, ac.TenantID, clientID, ac.UserID, reqScopes, ac.AMR, std, custom)

			// SYS namespace a partir de metadata + RBAC (Fase 2)
			if u, err := activeStore.GetUserByID(ctx, ac.UserID); err == nil && u != nil {
				type rbacReader interface {
					GetUserRoles(ctx context.Context, userID string) ([]string, error)
					GetUserPermissions(ctx context.Context, userID string) ([]string, error)
				}
				var roles, perms []string
				if rr, ok := activeStore.(rbacReader); ok {
					roles, _ = rr.GetUserRoles(ctx, ac.UserID)
					perms, _ = rr.GetUserPermissions(ctx, ac.UserID)
				}
				custom = helpers.PutSystemClaimsV2(custom, c.Issuer.Iss, u.Metadata, roles, perms)
			}

			access, exp, err := c.Issuer.IssueAccess(ac.UserID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 2210)
				return
			}

			// Refresh (rotación igual que en /v1/auth/*)
			var rawRT string
			if !hasStore {
				httpx.WriteError(w, http.StatusServiceUnavailable, "db_not_configured", "no hay base de datos configurada para emitir refresh tokens", 2212)
				return
			}

			type tc interface {
				CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
			}
			if tcs, ok := activeStore.(tc); ok {
				// preferir TC
				tok, err := tcs.CreateRefreshTokenTC(ctx, ac.TenantID, cl.ID, ac.UserID, refreshTTL)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2212)
					return
				}
				rawRT = tok // ya viene raw (tu TC genera el token)
			} else {
				// legacy
				var err error
				rawRT, err = tokens.GenerateOpaqueToken(32)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 2211)
					return
				}
				hash := tokens.SHA256Base64URL(rawRT)
				if _, err := activeStore.CreateRefreshToken(ctx, ac.UserID, cl.ID, hash, time.Now().Add(refreshTTL), nil); err != nil {
					log.Printf("oauth token: create refresh err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2212)
					return
				}
			}

			// ID Token (sin SYS_NS)
			idStd := map[string]any{
				"tid":     ac.TenantID,
				"at_hash": atHash(access),
				"azp":     clientID,
				"acr":     acrVal,
				"amr":     accessAMR, // añadir AMR al ID Token para interoperabilidad
			}
			idExtra := map[string]any{}
			if ac.Nonce != "" {
				idExtra["nonce"] = ac.Nonce
			}
			idStd, idExtra = applyIDClaimsHook(ctx, c, ac.TenantID, clientID, ac.UserID, reqScopes, ac.AMR, idStd, idExtra)

			idToken, _, err := c.Issuer.IssueIDToken(ac.UserID, clientID, idStd, idExtra)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el id_token", 2213)
				return
			}

			// Evitar cache
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			resp := map[string]any{
				"token_type":    "Bearer",
				"expires_in":    int64(time.Until(exp).Seconds()),
				"access_token":  access,
				"refresh_token": rawRT,
				"id_token":      idToken,
				"scope":         ac.Scope,
			}
			httpx.WriteJSON(w, http.StatusOK, resp)

		// ───────────────── refresh_token (rotación) ─────────────────
		case "refresh_token":
			clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
			refreshToken := strings.TrimSpace(r.PostForm.Get("refresh_token"))
			if clientID == "" || refreshToken == "" {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "client_id y refresh_token son obligatorios", 2220)
				return
			}

			ctx := r.Context()

			if !hasStore {
				httpx.WriteError(w, http.StatusServiceUnavailable, "db_not_configured", "no hay base de datos configurada para refrescar tokens", 2222)
				return
			}

			client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client not found", 2221)
				return
			}

			// Mapear client FS a estructura legacy para compatibilidad
			cl := &core.Client{
				ID:           client.ClientID,
				TenantID:     tenantSlug,
				RedirectURIs: client.RedirectURIs,
				Scopes:       client.Scopes,
			}
			_ = tenantSlug

			type tcRefresh interface {
				CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
				RevokeRefreshTokensByUserClientTC(ctx context.Context, tenantID, clientID, userID string) (int64, error)
			}
			type legacyGet interface {
				GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*core.RefreshToken, error)
			}

			var rt *core.RefreshToken
			if _, ok := activeStore.(tcRefresh); ok {
				// Para TC el token es opaco raw (no hash). Validación depende de tu diseño.
				// Si mantenés hash-only, podés agregar GetRefreshTokenByRawTC(ctx, tenantID, clientID, raw) en PG.
				httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "refresh_token TC aún no soportado en GET/validate", 2222)
				return
			} else if lg, ok := activeStore.(legacyGet); ok {
				// legacy tal como hoy...
				hash := tokens.SHA256Base64URL(refreshToken)
				rt, err = lg.GetRefreshTokenByHash(ctx, hash)
				if err != nil {
					status := http.StatusInternalServerError
					if err == core.ErrNotFound {
						status = http.StatusBadRequest
					}
					httpx.WriteError(w, status, "invalid_grant", "refresh inválido", 2222)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusServiceUnavailable, "store_not_supported", "store no soporta refresh tokens", 2222)
				return
			}
			now := time.Now()
			if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) || rt.ClientID != cl.ID {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "refresh revocado/expirado o mismatched client", 2223)
				return
			}

			std := map[string]any{
				"tid": cl.TenantID,
				"amr": []string{"refresh"},
				"acr": "urn:hellojohn:loa:1",
				"scp": []string{}, // refresh flow: sin scopes explícitos aquí
			}
			custom := map[string]any{}

			std, custom = applyAccessClaimsHook(ctx, c, cl.TenantID, clientID, rt.UserID, []string{}, []string{"refresh"}, std, custom)
			if u, err := activeStore.GetUserByID(ctx, rt.UserID); err == nil && u != nil {
				type rbacReader interface {
					GetUserRoles(ctx context.Context, userID string) ([]string, error)
					GetUserPermissions(ctx context.Context, userID string) ([]string, error)
				}
				var roles, perms []string
				if rr, ok := activeStore.(rbacReader); ok {
					roles, _ = rr.GetUserRoles(ctx, rt.UserID)
					perms, _ = rr.GetUserPermissions(ctx, rt.UserID)
				}
				custom = helpers.PutSystemClaimsV2(custom, c.Issuer.Iss, u.Metadata, roles, perms)
			}

			access, exp, err := c.Issuer.IssueAccess(rt.UserID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir access", 2224)
				return
			}

			var newRT string
			if tcs, ok := activeStore.(tcRefresh); ok {
				// TC: revocar tokens del usuario+cliente y crear uno nuevo
				_, _ = tcs.RevokeRefreshTokensByUserClientTC(ctx, cl.TenantID, cl.ID, rt.UserID)
				newRT, err = tcs.CreateRefreshTokenTC(ctx, cl.TenantID, cl.ID, rt.UserID, refreshTTL)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh TC", 2226)
					return
				}
			} else {
				// legacy
				newRT, err = tokens.GenerateOpaqueToken(32)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 2225)
					return
				}
				newHash := tokens.SHA256Base64URL(newRT)
				if _, err := activeStore.CreateRefreshToken(ctx, rt.UserID, cl.ID, newHash, now.Add(refreshTTL), &rt.ID); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2226)
					return
				}
				_ = activeStore.RevokeRefreshToken(ctx, rt.ID)
			}

			// Evitar cache
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"token_type":    "Bearer",
				"expires_in":    int64(time.Until(exp).Seconds()),
				"access_token":  access,
				"refresh_token": newRT,
			})

		default:
			httpx.WriteError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type no soportado", 2202)
		}
	}
}
