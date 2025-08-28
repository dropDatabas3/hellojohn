package handlers

import (
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

func NewOAuthRevokeHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		if err := r.ParseForm(); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "form inválido", 2301)
			return
		}
		token := strings.TrimSpace(r.PostForm.Get("token"))
		if token == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "token es obligatorio", 2302)
			return
		}
		// token_type_hint puede venir o no; ignoramos para idempotencia
		// Semántica: si existe y es refresh, lo revocamos; si no, igual 200.
		hash := tokens.SHA256Base64URL(token)
		if rt, err := c.Store.GetRefreshTokenByHash(r.Context(), hash); err == nil && rt != nil {
			_ = c.Store.RevokeRefreshToken(r.Context(), rt.ID)
		} else if err != nil && err != core.ErrNotFound {
			// errores inesperados no deben filtrar información
		}
		// RFC 7009: 200 OK siempre que el input sea bien formado
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.WriteHeader(http.StatusOK)
	}
}
