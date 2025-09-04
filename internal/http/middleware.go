package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ─────────────── CORS ───────────────
func WithCORS(next http.Handler, allowed []string) http.Handler {
	trim := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

	alist := make([]string, len(allowed))
	for i, v := range allowed {
		alist[i] = trim(v)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := trim(r.Header.Get("Origin"))
		allowedOrigin := ""

		for _, a := range alist {
			if a == "*" || (origin != "" && strings.EqualFold(origin, a)) {
				allowedOrigin = origin
				break
			}
		}

		// Ayuda a caches/proxies
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")

		if allowedOrigin != "" {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allowedOrigin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET,POST,HEAD,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			// DX: permitir leer desafios 401/redirects desde fetch()
			// Exponemos también Retry-After para clientes browser y headers de rate limiting
			h.Set("Access-Control-Expose-Headers", "X-Request-ID, X-RateLimit-Remaining, X-RateLimit-Limit, X-RateLimit-Reset, Retry-After, WWW-Authenticate, Location")
			h.Set("Access-Control-Max-Age", "600") // preflight 10m
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─────────────── Security Headers ───────────────

// isHTTPS intenta detectar si el request llegó por HTTPS (directo o detrás de proxy).
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

// WithSecurityHeaders inyecta cabeceras de defensa por defecto.
// No rompe endpoints porque no toca Cache-Control (eso ya lo maneja cada handler sensible a tokens).
func WithSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Referrer y MIME sniffing
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		w.Header().Set("X-DNS-Prefetch-Control", "off")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")

		// Clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// CSP estricta para API (no servimos HTML estático acá)
		// - default-src 'none' bloquea cualquier carga
		// - frame-ancestors 'none' refuerza anti-embebidos
		// - base-uri 'none' evita base href
		// - form-action 'self' permite POST directos si algún día hay formularios 1st-party
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")

		// Permissions-Policy: deshabilitamos superficies no usadas por una API
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")

		// HSTS solo si estamos en HTTPS (directo o detrás de proxy)
		if isHTTPS(r) {
			// 180 días, incluye subdominios (preload lo dejamos a decisión operativa)
			w.Header().Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// ─────────────── Request ID ───────────────
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if rid == "" {
			var b [16]byte
			_, _ = rand.Read(b[:])
			rid = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r)
	})
}

// ─────────────── Recover de pánicos ───────────────
func WithRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				rid := w.Header().Get("X-Request-ID")
				log.Printf(`{"level":"error","msg":"panic","request_id":"%s","recover":"%v"}`, rid, rec)
				WriteError(w, http.StatusInternalServerError, "internal_error", "panic recover", 1500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─────────────── Logging JSON ───────────────
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		dur := time.Since(start)

		rid := w.Header().Get("X-Request-ID")
		log.Printf(
			`{"level":"info","msg":"http","request_id":"%s","method":"%s","path":"%s","status":%d,"bytes":%d,"duration_ms":%d}`,
			rid, r.Method, r.URL.Path, rec.status, rec.bytes, dur.Milliseconds(),
		)
	})
}

// ─────────────── Rate Limit ───────────────
// Interface mínima para evitar dependencias aquí.
// (La implementación Redis está en internal/rate.)
type RateLimiter interface {
	Allow(ctx context.Context, key string) (struct {
		Allowed     bool
		Remaining   int64
		RetryAfter  time.Duration
		WindowTTL   time.Duration
		CurrentHits int64
	}, error)
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// lee hasta max bytes del body (si es JSON) para extraer un campo y repone el body
func extractJSONField(r *http.Request, field string, max int64) string {
	if r.Method != http.MethodPost ||
		!strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return ""
	}
	var buf bytes.Buffer
	_, _ = io.CopyN(&buf, r.Body, max)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))

	var tmp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &tmp); err == nil {
		if v, ok := tmp[field]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func rateKey(r *http.Request) string {
	ip := clientIP(r)
	path := r.URL.Path
	clientID := extractJSONField(r, "client_id", 4096)
	if clientID == "" {
		clientID = "-"
	}
	return ip + "|" + path + "|" + clientID
}

func WithRateLimit(next http.Handler, limiter RateLimiter) http.Handler {
	if limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// whitelist: no contar /healthz ni /readyz en el rate limit
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		key := rateKey(r)
		res, err := limiter.Allow(r.Context(), key)
		if err != nil {
			log.Printf(`{"level":"warn","msg":"rate_limit_error","err":"%v"}`, err)
			next.ServeHTTP(w, r)
			return
		}
		if !res.Allowed {
			if res.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(res.RetryAfter.Seconds())))
			}
			if res.WindowTTL > 0 {
				resetAt := time.Now().Add(res.WindowTTL).Unix()
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
			}
			WriteError(w, http.StatusTooManyRequests, "rate_limited", "demasiadas solicitudes", 1401)
			return
		}
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
		if res.WindowTTL > 0 {
			resetAt := time.Now().Add(res.WindowTTL).Unix()
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
		}
		next.ServeHTTP(w, r)
	})
}

// Adaptador de Limiter real a la interfaz local (si tu limiter es *rate.RedisLimiter)
type RateLimiterAdapter struct {
	inner interface {
		Allow(ctx context.Context, key string) (struct {
			Allowed     bool
			Remaining   int64
			RetryAfter  time.Duration
			WindowTTL   time.Duration
			CurrentHits int64
		}, error)
	}
}

func (a *RateLimiterAdapter) Allow(ctx context.Context, key string) (struct {
	Allowed     bool
	Remaining   int64
	RetryAfter  time.Duration
	WindowTTL   time.Duration
	CurrentHits int64
}, error) {
	return a.inner.Allow(ctx, key)
}
