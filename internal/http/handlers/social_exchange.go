package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

type SocialExchangeRequest struct {
	Code     string `json:"code"`
	ClientID string `json:"client_id"`
	TenantID string `json:"tenant_id,omitempty"`
}

func NewSocialExchangeHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1655)
			return
		}
		var req SocialExchangeRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.Code = strings.TrimSpace(req.Code)
		if req.Code == "" || strings.TrimSpace(req.ClientID) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "faltan parámetros", 1656)
			return
		}
		key := "social:code:" + req.Code
		payload, ok := c.Cache.Get(key)
		if !ok || len(payload) == 0 {
			httpx.WriteError(w, http.StatusNotFound, "code_not_found", "código inválido o expirado", 1657)
			return
		}

		// payload guardado: {client_id, tenant_id, response}
		var stored struct {
			ClientID string            `json:"client_id"`
			TenantID string            `json:"tenant_id"`
			Response AuthLoginResponse `json:"response"`
		}
		if err := json.Unmarshal(payload, &stored); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "payload_invalid", "payload inválido", 1658)
			return
		}
		if os.Getenv("SOCIAL_DEBUG_LOG") == "1" {
			log.Printf(`{"level":"debug","msg":"social_exchange_payload","req_client":"%s","stored_client":"%s","stored_tenant":"%s","code":"%s"}`,
				strings.TrimSpace(req.ClientID), strings.TrimSpace(stored.ClientID), stored.TenantID, req.Code)
		}
		// Validar client_id (requerido) y tenant_id (si provisto)
		if !strings.EqualFold(strings.TrimSpace(stored.ClientID), strings.TrimSpace(req.ClientID)) {
			// Debug opcional para investigar por qué el client_id no coincide en E2E (controlado por SOCIAL_DEBUG_LOG=1)
			if os.Getenv("SOCIAL_DEBUG_LOG") == "1" {
				log.Printf(`{"level":"debug","msg":"social_exchange_client_mismatch","req_client":"%s","stored_client":"%s","code":"%s","time":"%s"}`,
					strings.TrimSpace(req.ClientID), strings.TrimSpace(stored.ClientID), req.Code, time.Now().Format(time.RFC3339))
			}
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "client_id no coincide para este code", 1659)
			return
		}
		if strings.TrimSpace(req.TenantID) != "" && !strings.EqualFold(strings.TrimSpace(req.TenantID), strings.TrimSpace(stored.TenantID)) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "tenant_id no coincide para este code", 1660)
			return
		}

		// Antes de emitir la respuesta final, si el access venía con AMR base (p.e. solo google) y el usuario tiene MFA configurada
		// deberíamos bifurcar igual que en login/password: devolver mfa_required.
		// Detectamos userID a partir del refresh? No lo tenemos aquí; en el flujo social el response ya contiene tokens.
		// Simplificación: pedimos un token introspect? Evitamos overhead; en su lugar ampliamos el payload almacenado para incluir user_id (futuro). Por ahora asumimos que Google callback ya manejó MFA.
		// Para mantener consistencia con password login cuando se usa login_code se replicará lógica mínima si añadimos user_id en payload futuro.

		// 1 uso: eliminar solo tras validación exitosa
		c.Cache.Delete(key)

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		httpx.WriteJSON(w, http.StatusOK, stored.Response)
	}
}
