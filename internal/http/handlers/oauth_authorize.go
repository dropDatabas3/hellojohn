package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type authCode struct {
	UserID          string    `json:"user_id"`
	TenantID        string    `json:"tenant_id"`
	ClientID        string    `json:"client_id"`
	RedirectURI     string    `json:"redirect_uri"`
	Scope           string    `json:"scope"`
	Nonce           string    `json:"nonce"`
	CodeChallenge   string    `json:"code_challenge"`
	ChallengeMethod string    `json:"code_challenge_method"`
	AMR             []string  `json:"amr"`
	ExpiresAt       time.Time `json:"expires_at"`
}

func addQS(u, k, v string) string {
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + url.QueryEscape(k) + "=" + url.QueryEscape(v)
}

func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	loc := addQS(redirectURI, "error", code)
	if desc != "" {
		loc = addQS(loc, "error_description", desc)
	}
	if state != "" {
		loc = addQS(loc, "state", state)
	}
	http.Redirect(w, r, loc, http.StatusFound)
}

// soporta cookie de sesión y (opcional) Bearer como fallback
func NewOAuthAuthorizeHandler(c *app.Container, cookieName string, allowBearer bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		w.Header().Add("Vary", "Cookie")
		if allowBearer {
			w.Header().Add("Vary", "Authorization")
		}

		// Resolver store con precedencia: tenantDB > globalDB (para consent y magic link)
		// var activeScopesConsents core.ScopesConsentsRepository // Unused for now
		var activeStore core.Repository

		if c.TenantSQLManager != nil {
			// Intentar obtener store del tenant actual
			tenantSlug := cpctx.ResolveTenant(r)
			if tenantStore, err := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); err == nil && tenantStore != nil {
				activeStore = tenantStore
				// Para tenant stores, por ahora usar fallback a global consent (TODO: implementar per-tenant consent)
			}
		}
		// Fallback a global consents
		/* if c.ScopesConsents != nil {
			activeScopesConsents = c.ScopesConsents
		} */
		if activeStore == nil && c.Store != nil {
			activeStore = c.Store
		}

		q := r.URL.Query()
		responseType := strings.TrimSpace(q.Get("response_type"))
		clientID := strings.TrimSpace(q.Get("client_id"))
		redirectURI := strings.TrimSpace(q.Get("redirect_uri"))
		scope := strings.TrimSpace(q.Get("scope"))
		state := strings.TrimSpace(q.Get("state"))
		nonce := strings.TrimSpace(q.Get("nonce"))
		codeChallenge := strings.TrimSpace(q.Get("code_challenge"))
		codeMethod := strings.TrimSpace(q.Get("code_challenge_method"))

		if responseType != "code" || clientID == "" || redirectURI == "" || scope == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "faltan parámetros obligatorios", 2101)
			return
		}
		if !strings.Contains(scope, "openid") {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_scope", "scope debe incluir openid", 2102)
			return
		}
		if !strings.EqualFold(codeMethod, "S256") || codeChallenge == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "PKCE S256 requerido", 2103)
			return
		}

		ctx := r.Context()

		client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client", "client not found", 2104)
			return
		}
		if err := helpers.ValidateRedirectURI(client, redirectURI); err != nil {
			// If redirect_uri is present but doesn't match, RFC says invalid_redirect_uri
			httpx.WriteError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri not allowed for this client", 2105)
			return
		}

		// Validar scopes solicitados
		if scope := strings.TrimSpace(scope); scope != "" {
			for _, s := range strings.Fields(scope) {
				if !controlplane.DefaultIsScopeAllowed(client, s) {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_scope", "scope not allowed", 2106)
					return
				}
			}
		}

		// Continuar con lógica existente
		_ = tenantSlug

		// Mapear client FS a estructura legacy para compatibilidad
		cl := &core.Client{
			ID:           client.ClientID, // Usar clientID como ID temporal
			TenantID:     tenantSlug,      // Usar tenantSlug como TenantID
			RedirectURIs: client.RedirectURIs,
			Scopes:       client.Scopes,
		}
		// reqScopes removed as it was unused

		var (
			sub             string
			tid             string
			amr             []string
			trustedByCookie bool
		)

		// 1) Sesión COOKIE
		if ck, err := r.Cookie(cookieName); err == nil && ck != nil && strings.TrimSpace(ck.Value) != "" {
			key := "sid:" + tokens.SHA256Base64URL(ck.Value)
			log.Printf("DEBUG: authorize cookie found: %s", ck.Name)
			if b, ok := c.Cache.Get(key); ok {
				var sp SessionPayload
				_ = json.Unmarshal(b, &sp)
				log.Printf("DEBUG: authorize session payload: %+v | expected tenant: %s", sp, cl.TenantID)

				// Compare carefully
				if time.Now().Before(sp.Expires) && strings.EqualFold(sp.TenantID, cl.TenantID) {
					sub = sp.UserID
					tid = sp.TenantID
					amr = []string{"pwd"}
					log.Printf("DEBUG: authorize session valid! sub=%s tid=%s", sub, tid)
				} else {
					log.Printf("DEBUG: authorize session invalid (expired=%v, tenant_match=%v, sp_tenant='%s', cl_tenant='%s')",
						!time.Now().Before(sp.Expires), strings.EqualFold(sp.TenantID, cl.TenantID), sp.TenantID, cl.TenantID)
				}
			} else {
				log.Printf("DEBUG: authorize session cache miss for key: %s (truncated)", key[:10])
			}
		} else {
			log.Printf("DEBUG: authorize NO cookie found (err=%v)", err)
		}

		// 2) Fallback Bearer
		if sub == "" && allowBearer {
			ah := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(ah), "bearer ") {
				raw := strings.TrimSpace(ah[len("Bearer "):])
				tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(),
					jwtv5.WithValidMethods([]string{"EdDSA"}), jwtv5.WithIssuer(c.Issuer.Iss))
				if err == nil && tk.Valid {
					if claims, ok := tk.Claims.(jwtv5.MapClaims); ok {
						sub, _ = claims["sub"].(string)
						tid, _ = claims["tid"].(string)
						if v, ok := claims["amr"].([]any); ok {
							for _, i := range v {
								if s, ok := i.(string); ok {
									amr = append(amr, s)
								}
							}
						}
					}
				}
			}
		}

		if sub == "" || tid == "" || !strings.EqualFold(tid, cl.TenantID) {
			// Si prompt=none, retornamos error (interacción no permitida)
			if strings.Contains(r.URL.Query().Get("prompt"), "none") {
				redirectError(w, r, redirectURI, state, "login_required", "requiere login")
				return
			}

			// Redirigir al Login UI
			uiBase := os.Getenv("UI_BASE_URL")
			if uiBase == "" {
				uiBase = "http://localhost:3000"
			}

			// Construir return_to con la URL actual de authorize
			returnTo := r.URL.String() // path + query
			// Asegurar que returnTo tenga el host si falta (r.URL.String() puede ser relativo)
			if !r.URL.IsAbs() {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				host := r.Host
				if host == "" {
					host = "localhost:8080"
				}
				returnTo = fmt.Sprintf("%s://%s%s", scheme, host, r.URL.RequestURI())
			}

			loginURL := fmt.Sprintf("%s/login?return_to=%s", uiBase, url.QueryEscape(returnTo))
			http.Redirect(w, r, loginURL, http.StatusFound)
			return
		}

		// Step-up MFA: si el usuario tiene MFA TOTP confirmada y la AMR actual solo contiene pwd, devolver JSON mfa_required
		if len(amr) == 1 && amr[0] == "pwd" && activeStore != nil {
			// intentamos detectar secreto MFA
			if mg, ok := activeStore.(interface {
				GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
			}); ok {
				if m, _ := mg.GetMFATOTP(r.Context(), sub); m != nil && m.ConfirmedAt != nil {
					// Revisar trusted device cookie (si existe) para posiblemente elevar amr automáticamente
					if ck, err := r.Cookie("mfa_trust"); err == nil && ck != nil {
						if tc, ok2 := activeStore.(interface {
							IsTrustedDevice(ctx context.Context, userID, deviceHash string, now time.Time) (bool, error)
						}); ok2 {
							if ok3, _ := tc.IsTrustedDevice(r.Context(), sub, tokens.SHA256Base64URL(ck.Value), time.Now()); ok3 {
								trustedByCookie = true
							}
						}
					}
					if !trustedByCookie { // requerir desafío MFA antes de emitir code
						ch := mfaChallenge{
							UserID:   sub,
							TenantID: tid,
							ClientID: clientID,
							AMRBase:  []string{"pwd"},
							Scope:    strings.Fields(scope),
						}
						mid, _ := tokens.GenerateOpaqueToken(24)

						// Store challenge in cache
						b, _ := json.Marshal(ch)
						key := "mfa_req:" + mid
						c.Cache.Set(key, b, 5*time.Minute)

						// Return json instructing UI to show MFA entry
						resp := map[string]string{
							"status":    "mfa_required",
							"mfa_token": mid,
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK) // 200 OK so UI handles it
						json.NewEncoder(w).Encode(resp)
						return
					}
					// if trustedByCookie, we upgrade AMR implicitly
					amr = append(amr, "mfa", "totp")
				}
			}
		}

		// Generar Authorization Code
		code, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "code gen failed", 2107)
			return
		}

		ac := authCode{
			UserID:          sub,
			TenantID:        tid,
			ClientID:        clientID,
			RedirectURI:     redirectURI,
			Scope:           scope,
			Nonce:           nonce,
			CodeChallenge:   codeChallenge,
			ChallengeMethod: codeMethod,
			AMR:             amr,
			ExpiresAt:       time.Now().Add(10 * time.Minute),
		}
		b, _ := json.Marshal(ac)
		// Guardar code en cache (usamos prefijo code:)
		c.Cache.Set("code:"+code, b, 10*time.Minute)

		log.Printf("DEBUG: authorize success, redirecting to %s", redirectURI)

		// Success Redirect
		loc := addQS(redirectURI, "code", code)
		if state != "" {
			loc = addQS(loc, "state", state)
		}

		http.Redirect(w, r, loc, http.StatusFound)
	}
}
