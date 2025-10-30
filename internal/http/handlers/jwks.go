package handlers

import (
	"net/http"
	"regexp"
	"strings"

	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// JWKSHandler expone endpoints JWKS globales y por tenant, usando cache por tenant.
type JWKSHandler struct {
	Cache *jwtx.JWKSCache
}

func NewJWKSHandler(cache *jwtx.JWKSCache) *JWKSHandler { return &JWKSHandler{Cache: cache} }

// GetGlobal maneja GET/HEAD /.well-known/jwks.json
func (h *JWKSHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/HEAD", 1001)
		return
	}
	setNoStore(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	data, err := h.Cache.Get("global")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 1501)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// GetByTenant maneja GET/HEAD /.well-known/jwks/{slug}.json
func (h *JWKSHandler) GetByTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/HEAD", 1001)
		return
	}
	// Extraer slug del path (stdlib ServeMux)
	const prefix = "/.well-known/jwks/"
	path := r.URL.Path
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, ".json") {
		http.NotFound(w, r)
		return
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(path, prefix), ".json")
	if !regexp.MustCompile(`^[a-z0-9\-]{1,64}$`).MatchString(slug) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid slug", 1502)
		return
	}
	setNoStore(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	data, err := h.Cache.Get(slug)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 1501)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
