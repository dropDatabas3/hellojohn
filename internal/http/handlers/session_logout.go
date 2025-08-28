package handlers

import (
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

// Usamos BuildDeletionCookie(name, domain, sameSite, secure) definido en cookieutil.go
func NewSessionLogoutHandler(c *app.Container, cookieName, cookieDomain, sameSite string, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		ck, err := r.Cookie(cookieName)
		if err == nil && ck != nil && strings.TrimSpace(ck.Value) != "" {
			// Borrar server-side
			key := "sid:" + tokensSHA256(ck.Value)
			c.Cache.Delete(key)
			// Borrar client-side
			del := BuildDeletionCookie(cookieName, cookieDomain, sameSite, secure)
			http.SetCookie(w, del)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
