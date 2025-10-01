package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

// NewCSRFGetHandler returns a GET handler that issues a CSRF token via cookie and JSON body.
// Cookie attributes: SameSite=Lax, HttpOnly=false, Path=/, short TTL (e.g., 30m).
// Response: {"csrf_token":"..."} with Cache-Control: no-store.
func NewCSRFGetHandler(cookieName string, ttl time.Duration) http.HandlerFunc {
	if cookieName == "" {
		cookieName = "csrf_token"
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		// generate random token
		var b [32]byte
		_, _ = rand.Read(b[:])
		tok := hex.EncodeToString(b[:])
		exp := time.Now().Add(ttl).UTC()

		// non-HttpOnly by design so frontend can read it (double-submit); SameSite Lax
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    tok,
			Path:     "/",
			HttpOnly: false,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			Expires:  exp,
		})

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"csrf_token": tok})
	}
}
