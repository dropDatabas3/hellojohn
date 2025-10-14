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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/claims"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

//
// ───────────────────────────── CORS ─────────────────────────────
//

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
			// Exponer headers útiles a fetch()
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

//
// ──────────────────────── Security Headers ───────────────────────
//

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

		// CSP estricta para API (no servimos HTML)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")

		// Exponer headers de debug de blacklist si corresponde
		if os.Getenv("DEBUG_PASSWORD_BLACKLIST") == "1" {
			if w.Header().Get("Access-Control-Expose-Headers") != "" &&
				!strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), "X-Debug-Blacklist-Path") {
				w.Header().Set("Access-Control-Expose-Headers",
					w.Header().Get("Access-Control-Expose-Headers")+", X-Debug-Blacklist-Path, X-Debug-Blacklist-Hit, X-Debug-Blacklist-Err")
			}
		}

		// Permissions-Policy: deshabilitar superficies no usadas por una API
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")

		// HSTS si HTTPS
		if isHTTPS(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

//
// ────────────────────────── Request ID ───────────────────────────
//

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

//
// ───────────────────────── Recover panic ─────────────────────────
//

func WithRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				rid := w.Header().Get("X-Request-ID")
				log.Printf(`{"level":"error","msg":"panic","request_id":"%s","path":"%s","method":"%s","recover":"%v"}`, rid, r.URL.Path, r.Method, rec)
				WriteError(w, http.StatusInternalServerError, "internal_error", "panic recover", 1500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

//
// ──────────────────────────── Logging ────────────────────────────
//

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHeader {
		return // Evitar llamadas múltiples
	}
	s.status = code
	s.wroteHeader = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.status = http.StatusOK
		s.wroteHeader = true
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

//
// ────────────────────────── Rate Limit ───────────────────────────
//

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

// ─────────────────────── Auth / Admin (NEW) ──────────────────────

// Middleware es un decorador de http.Handler
type Middleware func(http.Handler) http.Handler

// Chain aplica middlewares en orden (Chain(h, A, B) => A(B(h)))
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type ctxKey string

const (
	ctxClaimsKey ctxKey = "claims"
)

// GetClaims obtiene claims del contexto (las setea RequireAuth)
func GetClaims(ctx context.Context) map[string]any {
	if v := ctx.Value(ctxClaimsKey); v != nil {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func claimString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// RequireAuth valida Authorization: Bearer <JWT>, firma/iss con el keystore
// y guarda las claims en el contexto para handlers posteriores.
func RequireAuth(issuer *jwtx.Issuer) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ah := strings.TrimSpace(r.Header.Get("Authorization"))
			if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
				w.Header().Set("WWW-Authenticate", `Bearer realm="admin", error="invalid_token", error_description="missing bearer token"`)
				WriteError(w, http.StatusUnauthorized, "missing_bearer", "falta Authorization: Bearer <token>", 4010)
				return
			}
			raw := strings.TrimSpace(ah[len("Bearer "):])

			claims, err := jwtx.ParseEdDSA(raw, issuer.Keys, issuer.Iss)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="admin", error="invalid_token", error_description="`+err.Error()+`"`)
				WriteError(w, http.StatusUnauthorized, "invalid_token", "token inválido o expirado", 4011)
				return
			}

			ctx := context.WithValue(r.Context(), ctxClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSysAdmin valida admin del SISTEMA usando el namespace anclado al issuer.
// Reglas:
//  1. Si ADMIN_ENFORCE != "1": permitir (modo dev/compat).
//  2. Leer custom[SYS_NS].is_admin == true  => permitir.
//  3. Leer custom[SYS_NS].roles incluye "sys:admin" => permitir.
//  4. Fallback de emergencia: sub ∈ ADMIN_SUBS => permitir.
func RequireSysAdmin(issuer *jwtx.Issuer) Middleware {
	enforce := strings.TrimSpace(os.Getenv("ADMIN_ENFORCE")) == "1"
	adminSubs := map[string]struct{}{}
	if csv := strings.TrimSpace(os.Getenv("ADMIN_SUBS")); csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if s := strings.TrimSpace(p); s != "" {
				adminSubs[s] = struct{}{}
			}
		}
	}
	sysNS := claims.SystemNamespace(issuer.Iss)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enforce {
				next.ServeHTTP(w, r)
				return
			}
			cl := GetClaims(r.Context())
			if cl == nil {
				WriteError(w, http.StatusUnauthorized, "missing_claims", "no claims in context", 4012)
				return
			}
			if cust, ok := cl["custom"].(map[string]any); ok {
				if raw, ok := cust[sysNS]; ok {
					if sys, ok := raw.(map[string]any); ok {
						if v, ok := sys["is_admin"].(bool); ok && v {
							next.ServeHTTP(w, r)
							return
						}
						if rs, ok := sys["roles"].([]any); ok {
							for _, it := range rs {
								if s, ok := it.(string); ok && strings.EqualFold(s, "sys:admin") {
									next.ServeHTTP(w, r)
									return
								}
							}
						}
					}
				}
			}
			if sub := claimString(cl, "sub"); sub != "" { // Fallback ADMIN_SUBS
				if _, ok := adminSubs[sub]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			WriteError(w, http.StatusForbidden, "forbidden_admin", "admin requerido", 4030)
		})
	}
}

// RequireAdmin aplica la política de “quién es admin”.
// Reglas (en este orden):
//  1. Si ADMIN_ENFORCE != "1": permitir (modo compatible por defecto).
//  2. Si custom.is_admin == true  => permitir.
//  3. Si custom.roles incluye "admin" => permitir.
//  4. Si el sub (user id) está en ADMIN_SUBS (lista CSV) => permitir.
//     Si no, 403.
func RequireAdmin() Middleware {
	enforce := strings.TrimSpace(os.Getenv("ADMIN_ENFORCE")) == "1"
	adminSubs := map[string]struct{}{}
	if csv := strings.TrimSpace(os.Getenv("ADMIN_SUBS")); csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if s := strings.TrimSpace(p); s != "" {
				adminSubs[s] = struct{}{}
			}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enforce {
				next.ServeHTTP(w, r)
				return
			}
			claims := GetClaims(r.Context())
			if claims == nil {
				WriteError(w, http.StatusUnauthorized, "missing_claims", "no claims in context", 4012)
				return
			}
			// custom.is_admin
			if cust, ok := claims["custom"].(map[string]any); ok {
				if v, ok := cust["is_admin"].(bool); ok && v {
					next.ServeHTTP(w, r)
					return
				}
				// custom.roles: ["admin", ...]
				if arr, ok := cust["roles"].([]any); ok {
					for _, it := range arr {
						if s, ok := it.(string); ok && strings.EqualFold(s, "admin") {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}
			// Admin por SUB (fallback por env)
			if sub := claimString(claims, "sub"); sub != "" {
				if _, ok := adminSubs[sub]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			WriteError(w, http.StatusForbidden, "forbidden_admin", "admin requerido", 4030)
		})
	}
}

// RequireScope enforces that the access token contains the required scope.
// It expects RequireAuth to have already populated claims in context.
// Accepted claim shapes:
// - scp: ["scope1","scope2"] (preferred)
// - scope: "scope1 scope2" (space separated)
func RequireScope(want string) Middleware {
	want = strings.ToLower(strings.TrimSpace(want))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if want == "" {
				next.ServeHTTP(w, r)
				return
			}
			cl := GetClaims(r.Context())
			if cl == nil {
				WriteError(w, http.StatusUnauthorized, "missing_claims", "no claims in context", 4012)
				return
			}
			// collect scopes
			has := false
			if v, ok := cl["scp"]; ok {
				switch arr := v.(type) {
				case []any:
					for _, it := range arr {
						if s, ok := it.(string); ok && strings.EqualFold(strings.TrimSpace(s), want) {
							has = true
							break
						}
					}
				case []string:
					for _, s := range arr {
						if strings.EqualFold(strings.TrimSpace(s), want) {
							has = true
							break
						}
					}
				}
			}
			if !has {
				if s, ok := cl["scope"].(string); ok {
					for _, part := range strings.Fields(s) {
						if strings.EqualFold(strings.TrimSpace(part), want) {
							has = true
							break
						}
					}
				}
			}
			if !has {
				// RFC6750 suggests 403 with insufficient_scope
				w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+want+`"`)
				WriteError(w, http.StatusForbidden, "insufficient_scope", "scope requerido: "+want, 4031)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyScope enforces that the access token contains ANY of the required scopes.
// It expects RequireAuth to have already populated claims in context.
// Accepted claim shapes:
// - scp: ["scope1","scope2"] (preferred)
// - scope: "scope1 scope2" (space separated)
func RequireAnyScope(wants ...string) Middleware {
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	var need []string
	for _, w := range wants {
		if n := norm(w); n != "" {
			need = append(need, n)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(need) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			cl := GetClaims(r.Context())
			if cl == nil {
				WriteError(w, http.StatusUnauthorized, "missing_claims", "no claims in context", 4012)
				return
			}
			has := func(scope string) bool {
				// scp array
				if v, ok := cl["scp"]; ok {
					switch arr := v.(type) {
					case []any:
						for _, it := range arr {
							if s, ok := it.(string); ok && strings.EqualFold(strings.TrimSpace(s), scope) {
								return true
							}
						}
					case []string:
						for _, s := range arr {
							if strings.EqualFold(strings.TrimSpace(s), scope) {
								return true
							}
						}
					}
				}
				// scope string
				if s, ok := cl["scope"].(string); ok {
					for _, part := range strings.Fields(s) {
						if strings.EqualFold(strings.TrimSpace(part), scope) {
							return true
						}
					}
				}
				return false
			}
			ok := false
			for _, wnt := range need {
				if has(wnt) {
					ok = true
					break
				}
			}
			if !ok {
				w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+strings.Join(need, " ")+`"`)
				WriteError(w, http.StatusForbidden, "insufficient_scope", "scope requerido (alguno de): "+strings.Join(need, ", "), 4031)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireLeader asegura que las escrituras sólo se acepten en el líder.
// Comportamiento:
//   - Si no hay cluster o el nodo es líder ⇒ pasa.
//   - Si es follower ⇒ devuelve 409 con X-Leader=<nodeID>.
//   - Si existe LEADER_REDIRECTS[nodeID] y el cliente lo pide (header X-Leader-Redirect: 1
//     o query leader_redirect=1) ⇒ responde 307 Location hacia la URL del líder (mismo path/query).
func RequireLeader(c *app.Container) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Sólo aplica a métodos no idempotentes típicos de escritura
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				// continue
			default:
				next.ServeHTTP(w, r)
				return
			}
			if c == nil || c.ClusterNode == nil || c.ClusterNode.IsLeader() {
				next.ServeHTTP(w, r)
				return
			}
			leaderID := c.ClusterNode.LeaderID()
			if leaderID != "" {
				w.Header().Set("X-Leader", leaderID)
			}
			// ¿Pidió redirect explícito el cliente?
			wantsRedirect := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Leader-Redirect")), "1") ||
				strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("leader_redirect")), "1")
			if wantsRedirect && leaderID != "" && c.LeaderRedirects != nil {
				if base, ok := c.LeaderRedirects[leaderID]; ok && strings.TrimSpace(base) != "" {
					// Validar que base sea una URL absoluta http/https y que esté en whitelist explícita
					ub := strings.TrimSpace(base)
					if (strings.HasPrefix(strings.ToLower(ub), "http://") || strings.HasPrefix(strings.ToLower(ub), "https://")) && !strings.Contains(ub, " ") {
						// Construir Location conservando path y query
						ub = strings.TrimRight(ub, "/")
						loc := ub + r.URL.RequestURI()
						w.Header().Set("X-Leader-URL", ub)
						w.Header().Set("Location", loc)
						w.WriteHeader(http.StatusTemporaryRedirect) // 307
						return
					}
				}
			}
			// Fallback: 409 con error estándar
			WriteError(w, http.StatusConflict, "not_leader", "este nodo es follower", 4001)
		})
	}
}

// RequireCSRF enforces a double-submit CSRF check for cookie-based requests.
// Behavior:
//   - If Authorization: Bearer is present, the check is skipped (non-cookie flow).
//   - Otherwise, for unsafe methods (POST, PUT, PATCH, DELETE), it requires a matching
//     CSRF header and cookie with the same value.
//   - Header/cookie names are configurable per handler wiring; this middleware only checks and returns 403 on mismatch.
func RequireCSRF(headerName, cookieName string) Middleware {
	hn := strings.TrimSpace(headerName)
	if hn == "" {
		hn = "X-CSRF-Token"
	}
	cn := strings.TrimSpace(cookieName)
	if cn == "" {
		cn = "csrf_token"
	}
	isUnsafe := func(m string) bool {
		switch strings.ToUpper(m) {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			return true
		default:
			return false
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isUnsafe(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			// Skip CSRF if Bearer auth is present (not a cookie flow)
			if ah := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(ah), "bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			// Read header and cookie
			hdr := strings.TrimSpace(r.Header.Get(hn))
			ck, _ := r.Cookie(cn)
			if hdr == "" || ck == nil || strings.TrimSpace(ck.Value) == "" || !subtleEqualCI(hdr, ck.Value) {
				// RFC-ish contract for CSRF error
				WriteError(w, http.StatusForbidden, "invalid_csrf_token", "CSRF token missing or mismatch", 1600)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// subtleEqualCI compares two strings for equality in constant time after exact match (case sensitive).
// CSRF token comparison should be exact; keep this a simple constant-time eq for same length.
func subtleEqualCI(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
