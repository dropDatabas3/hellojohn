package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
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
		cl, _, err := c.Store.GetClientByClientID(ctx, clientID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			httpx.WriteError(w, status, "invalid_client", "client inválido", 2104)
			return
		}
		okRedirect := false
		for _, ru := range cl.RedirectURIs {
			if ru == redirectURI {
				okRedirect = true
				break
			}
		}
		if !okRedirect {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri no coincide con el client", 2105)
			return
		}
		reqScopes := strings.Fields(scope)
		for _, s := range reqScopes {
			found := false
			for _, allowed := range cl.Scopes {
				if strings.EqualFold(s, allowed) {
					found = true
					break
				}
			}
			if !found {
				redirectError(w, r, redirectURI, state, "invalid_scope", "scope no permitido para este client")
				return
			}
		}

		var (
			sub             string
			tid             string
			amr             []string
			trustedByCookie bool
		)

		// 1) Sesión COOKIE
		if ck, err := r.Cookie(cookieName); err == nil && ck != nil && strings.TrimSpace(ck.Value) != "" {
			key := "sid:" + tokens.SHA256Base64URL(ck.Value)
			if b, ok := c.Cache.Get(key); ok {
				var sp SessionPayload
				_ = json.Unmarshal(b, &sp)
				if time.Now().Before(sp.Expires) && sp.TenantID == cl.TenantID {
					sub = sp.UserID
					tid = sp.TenantID
					amr = []string{"pwd"}
				}
			}
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
			redirectError(w, r, redirectURI, state, "login_required", "requiere login")
			return
		}

		// Step-up MFA: si el usuario tiene MFA TOTP confirmada y la AMR actual solo contiene pwd, devolver JSON mfa_required
		if len(amr) == 1 && amr[0] == "pwd" {
			// intentamos detectar secreto MFA
			if mg, ok := c.Store.(interface {
				GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
			}); ok {
				if m, _ := mg.GetMFATOTP(r.Context(), sub); m != nil && m.ConfirmedAt != nil {
					// Revisar trusted device cookie (si existe) para posiblemente elevar amr automáticamente
					if ck, err := r.Cookie("mfa_trust"); err == nil && ck != nil {
						if tc, ok2 := c.Store.(interface {
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
						key := "mfa:token:" + mid
						buf, _ := json.Marshal(ch)
						c.Cache.Set(key, buf, 5*time.Minute)
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						w.Header().Set("Cache-Control", "no-store")
						w.Header().Set("Pragma", "no-cache")
						// 200 con indicador para frontend/SPA
						httpx.WriteJSON(w, http.StatusOK, map[string]any{
							"mfa_required": true,
							"mfa_token":    mid,
							"amr":          []string{"pwd"},
							"step_up":      true,
						})
						return
					} else {
						// trusted device => elevamos amr antes de continuar flujo normal
						amr = append(amr, "mfa")
					}
				}
			}
		}

		// ─────────────────────────────────────────────────────────────
		// Gate de consentimiento: si faltan scopes ⇒ respuesta JSON consent_required
		// Se ejecuta después de validar login y (posible) MFA, antes de generar authorization code.
		// ─────────────────────────────────────────────────────────────
		if c.ScopesConsents != nil {
			granted := []string{}
			if uc, err := c.ScopesConsents.GetConsent(ctx, sub, cl.ID); err == nil && uc.RevokedAt == nil {
				granted = uc.GrantedScopes
			}
			set := map[string]struct{}{}
			for _, g := range granted {
				g = strings.ToLower(strings.TrimSpace(g))
				if g != "" {
					set[g] = struct{}{}
				}
			}
			var missing []string
			for _, rs := range reqScopes {
				key := strings.ToLower(strings.TrimSpace(rs))
				if key == "" {
					continue
				}
				if _, ok := set[key]; !ok {
					missing = append(missing, rs)
				}
			}
			if len(missing) > 0 {
				// ==== Autoconsent opcional (scopes básicos) controlado por env ====
				auto := strings.TrimSpace(os.Getenv("CONSENT_AUTO"))
				if auto == "" {
					auto = "1"
				} // por defecto habilitado
				allowed := map[string]struct{}{"openid": {}, "email": {}, "profile": {}}
				if raw := strings.TrimSpace(os.Getenv("CONSENT_AUTO_SCOPES")); raw != "" {
					allowed = map[string]struct{}{}
					for _, s := range strings.Fields(raw) {
						s = strings.ToLower(strings.TrimSpace(s))
						if s != "" {
							allowed[s] = struct{}{}
						}
					}
				}
				subset := true
				for _, s := range reqScopes {
					if _, ok := allowed[strings.ToLower(strings.TrimSpace(s))]; !ok {
						subset = false
						break
					}
				}
				needConsentResponse := true
				if auto != "0" && subset {
					var upErr error
					type up1 interface {
						UpsertConsent(ctx context.Context, tenantID, userID, clientID string, scopes []string) error
					}
					type up2 interface {
						UpsertConsent(ctx context.Context, userID, clientID string, scopes []string) error
					}
					if u1, ok := c.ScopesConsents.(up1); ok {
						upErr = u1.UpsertConsent(ctx, tid, sub, cl.ID, reqScopes)
					} else if u2, ok := c.ScopesConsents.(up2); ok {
						upErr = u2.UpsertConsent(ctx, sub, cl.ID, reqScopes)
					}
					if upErr == nil {
						needConsentResponse = false // éxito: continuamos flujo normal
					}
				}
				if needConsentResponse { // emitir consent_required
					mid, _ := tokens.GenerateOpaqueToken(24)
					payload := consentChallenge{
						UserID:              sub,
						TenantID:            tid,
						ClientID:            cl.ID,
						RedirectURI:         redirectURI,
						State:               state,
						Nonce:               nonce,
						CodeChallenge:       codeChallenge,
						CodeChallengeMethod: "S256",
						RequestedScopes:     reqScopes,
						AMR:                 amr,
						ExpiresAt:           time.Now().Add(10 * time.Minute),
					}
					buf, _ := json.Marshal(payload)
					c.Cache.Set("consent:token:"+mid, buf, 10*time.Minute)

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Pragma", "no-cache")
					httpx.WriteJSON(w, http.StatusOK, map[string]any{
						"consent_required": true,
						"consent_token":    mid,
						"requested_scopes": reqScopes,
					})
					return
				}
			}
		}

		code, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo generar code", 2106)
			return
		}
		key := "oidc:code:" + tokens.SHA256Base64URL(code)
		payload := authCode{
			UserID:          sub,
			TenantID:        tid,
			ClientID:        cl.ID,
			RedirectURI:     redirectURI,
			Scope:           scope,
			Nonce:           nonce,
			CodeChallenge:   codeChallenge,
			ChallengeMethod: "S256",
			AMR:             amr,
			ExpiresAt:       time.Now().Add(5 * time.Minute),
		}
		b, _ := json.Marshal(payload)
		c.Cache.Set(key, b, 5*time.Minute)

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		loc := addQS(redirectURI, "code", code)
		if state != "" {
			loc = addQS(loc, "state", state)
		}
		http.Redirect(w, r, loc, http.StatusFound)
	}
}
