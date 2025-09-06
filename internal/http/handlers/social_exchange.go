package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

type SocialExchangeRequest struct {
	Code     string `json:"code"`
	ClientID string `json:"client_id"`
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
		// 1 uso
		c.Cache.Delete(key)

		// payload ya es AuthLoginResponse (emitido por callback/result)
		var resp AuthLoginResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "payload_invalid", "payload inválido", 1658)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		httpx.WriteJSON(w, http.StatusOK, resp)
	}
}
