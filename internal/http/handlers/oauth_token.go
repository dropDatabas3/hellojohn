package handlers

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
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
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
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
			cl, _, err := c.Store.GetClientByClientID(ctx, clientID)
			if err != nil {
				status := http.StatusInternalServerError
				if err == core.ErrNotFound {
					status = http.StatusUnauthorized
				}
				httpx.WriteError(w, status, "invalid_client", "client inválido", 2204)
				return
			}

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
			// Coherencia client/redirect_uri
			if ac.ClientID != clientID || ac.RedirectURI != redirectURI {
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
			std := map[string]any{
				"tid":   ac.TenantID,
				"amr":   ac.AMR,
				"scp":   reqScopes,
				"scope": ac.Scope, // compat
			}
			custom := map[string]any{}

			std, custom = applyAccessClaimsHook(ctx, c, ac.TenantID, clientID, ac.UserID, reqScopes, ac.AMR, std, custom)

			access, exp, err := c.Issuer.IssueAccess(ac.UserID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 2210)
				return
			}

			// Refresh (rotación igual que en /v1/auth/*)
			rawRT, err := tokens.GenerateOpaqueToken(32)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 2211)
				return
			}
			hash := tokens.SHA256Base64URL(rawRT)
			if _, err := c.Store.CreateRefreshToken(ctx, ac.UserID, cl.ID, hash, time.Now().Add(refreshTTL), nil); err != nil {
				log.Printf("oauth token: create refresh err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2212)
				return
			}

			// ID Token: std/extra (top-level). El at_hash depende del access recién emitido.
			idStd := map[string]any{
				"tid":     ac.TenantID,
				"at_hash": atHash(access),
				"azp":     clientID, // OIDC recomendado
			}
			idExtra := map[string]any{}
			if ac.Nonce != "" {
				idExtra["nonce"] = ac.Nonce
			}

			// Hook opcional (no puede pisar at_hash/azp/nonce/iss/sub/aud/*)
			idStd, idExtra = applyIDClaimsHook(ctx, c, ac.TenantID, clientID, ac.UserID, reqScopes, ac.AMR, idStd, idExtra)

			idToken, _, err := c.Issuer.IssueIDToken(ac.UserID, clientID, idStd, idExtra)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el id_token", 2213)
				return
			}

			// Evitar cache en respuestas con tokens
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
			cl, _, err := c.Store.GetClientByClientID(ctx, clientID)
			if err != nil {
				status := http.StatusInternalServerError
				if err == core.ErrNotFound {
					status = http.StatusUnauthorized
				}
				httpx.WriteError(w, status, "invalid_client", "client inválido", 2221)
				return
			}

			hash := tokens.SHA256Base64URL(refreshToken)
			rt, err := c.Store.GetRefreshTokenByHash(ctx, hash)
			if err != nil {
				status := http.StatusInternalServerError
				if err == core.ErrNotFound {
					status = http.StatusBadRequest
				}
				httpx.WriteError(w, status, "invalid_grant", "refresh inválido", 2222)
				return
			}
			now := time.Now()
			if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) || rt.ClientID != cl.ID {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_grant", "refresh revocado/expirado o mismatched client", 2223)
				return
			}

			// Access via refresh: amr=["refresh"], tid del client
			std := map[string]any{
				"tid": cl.TenantID,
				"amr": []string{"refresh"},
			}
			custom := map[string]any{}

			std, custom = applyAccessClaimsHook(ctx, c, cl.TenantID, clientID, rt.UserID, []string{}, []string{"refresh"}, std, custom)

			access, exp, err := c.Issuer.IssueAccess(rt.UserID, clientID, std, custom)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir access", 2224)
				return
			}

			newRT, err := tokens.GenerateOpaqueToken(32)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 2225)
				return
			}
			newHash := tokens.SHA256Base64URL(newRT)
			if _, err := c.Store.CreateRefreshToken(ctx, rt.UserID, cl.ID, newHash, now.Add(refreshTTL), &rt.ID); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 2226)
				return
			}
			_ = c.Store.RevokeRefreshToken(ctx, rt.ID)

			// Evitar cache en respuestas con tokens
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"token_type":    "Bearer",
				"expires_in":    int64(time.Until(exp).Seconds()),
				"access_token":  access,
				"refresh_token": newRT,
				// Por diseño, en refresh no devolvemos id_token.
			})

		default:
			httpx.WriteError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type no soportado", 2202)
		}
	}
}
