package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
)

type consentAcceptReq struct {
	Token   string `json:"consent_token"`
	Approve bool   `json:"approve"`
}

func NewConsentAcceptHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 2300)
			return
		}
		var in consentAcceptReq
		if !httpx.ReadJSON(w, r, &in) {
			return
		}
		in.Token = strings.TrimSpace(in.Token)
		if in.Token == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "consent_token requerido", 2301)
			return
		}

		key := "consent:token:" + in.Token
		raw, ok := c.Cache.Get(key)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "token inválido o expirado", 2302)
			return
		}
		// one-shot
		c.Cache.Delete(key)

		var payload consentChallenge
		if err := json.Unmarshal(raw, &payload); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "payload corrupto", 2303)
			return
		}
		if time.Now().After(payload.ExpiresAt) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "token expirado", 2304)
			return
		}

		if !in.Approve {
			// RFC: access_denied
			loc := addQS(payload.RedirectURI, "error", "access_denied")
			if payload.State != "" {
				loc = addQS(loc, "state", payload.State)
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			http.Redirect(w, r, loc, http.StatusFound)
			return
		}

		// Persistir consentimiento (Upsert con unión en store)
		if c.ScopesConsents == nil {
			httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "scopes/consents no soportado", 2305)
			return
		}
		if _, err := c.ScopesConsents.UpsertConsent(r.Context(),
			payload.UserID, payload.ClientID, payload.RequestedScopes,
		); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo guardar el consentimiento", 2306)
			return
		}

		// Emitir authorization code reutilizando PKCE/state/nonce
		code, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo generar code", 2307)
			return
		}
		ac := authCode{
			UserID:          payload.UserID,
			TenantID:        payload.TenantID,
			ClientID:        payload.ClientID,
			RedirectURI:     payload.RedirectURI,
			Scope:           strings.Join(payload.RequestedScopes, " "),
			Nonce:           payload.Nonce,
			CodeChallenge:   payload.CodeChallenge,
			ChallengeMethod: payload.CodeChallengeMethod,
			AMR:             payload.AMR,
			ExpiresAt:       time.Now().Add(5 * time.Minute),
		}
		buf, _ := json.Marshal(ac)
		c.Cache.Set("oidc:code:"+tokens.SHA256Base64URL(code), buf, 5*time.Minute)

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		loc := addQS(payload.RedirectURI, "code", code)
		if payload.State != "" {
			loc = addQS(loc, "state", payload.State)
		}
		http.Redirect(w, r, loc, http.StatusFound)
	}
}
