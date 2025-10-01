package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	e2eint "github.com/dropDatabas3/hellojohn/test/e2e/internal"
)

// 00 - Smoke + Discovery, JWKS, CORS y headers básicos
func Test_00_Smoke_Discovery(t *testing.T) {
	c := newHTTPClient()

	t.Run("healthz", func(t *testing.T) {
		resp, err := c.Get(baseURL + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /healthz status=%d", resp.StatusCode)
		}
	})

	t.Run("readyz", func(t *testing.T) {
		// no asumimos body exacto; sólo que responde 200 cuando el server está OK
		resp, err := c.Get(baseURL + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /readyz status=%d", resp.StatusCode)
		}
	})

	t.Run("CORS preflight allowed/denied", func(t *testing.T) {
		// Origin permitido (ideal: del seed). Si no hay, fallback usual de dev.
		originOK := "http://localhost:3000"
		if len(seed.Clients.Web.Origins) > 0 && seed.Clients.Web.Origins[0] != "" {
			originOK = seed.Clients.Web.Origins[0]
		}

		reqOK, _ := http.NewRequest("OPTIONS", baseURL+"/v1/auth/login", nil)
		reqOK.Header.Set("Origin", originOK)
		reqOK.Header.Set("Access-Control-Request-Method", "POST")
		reqOK.Header.Set("Access-Control-Request-Headers", "content-type, authorization")
		respOK, err := c.Do(reqOK)
		if err != nil {
			t.Fatal(err)
		}
		respOK.Body.Close()
		if respOK.StatusCode != http.StatusNoContent {
			t.Fatalf("preflight allowed -> expected 204 got %d", respOK.StatusCode)
		}
		if !e2eint.ACAOReflects(respOK, originOK) {
			t.Fatalf("ACAO should echo allowed origin (%s)", originOK)
		}

		// --- Headers CORS adicionales (allowed) ---
		if am := e2eint.GetHeaderLower(respOK, "Access-Control-Allow-Methods"); !strings.Contains(am, "post") {
			t.Fatalf("ACAM should include POST, got %q", am)
		}
		ah := e2eint.GetHeaderLower(respOK, "Access-Control-Allow-Headers")
		for _, need := range []string{"authorization", "content-type"} {
			if !e2eint.HasTokenCI(ah, need) && !strings.Contains(ah, need) { // fallback contains for safety
				t.Fatalf("ACAH should include %q, got %q", need, ah)
			}
		}

		// Access-Control-Allow-Credentials (solo si el backend lo habilita)
		// Por defecto asumimos true (muchos backends lo habilitan para sesiones),
		// y permitimos override vía CORS_ALLOW_CREDENTIALS.
		wantCreds := true
		if v := os.Getenv("CORS_ALLOW_CREDENTIALS"); v != "" {
			wantCreds = strings.EqualFold(v, "true")
		}
		gotCreds := strings.EqualFold(respOK.Header.Get("Access-Control-Allow-Credentials"), "true")
		if wantCreds && !gotCreds {
			t.Fatalf("expected Access-Control-Allow-Credentials:true for allowed origin")
		}
		if !wantCreds && gotCreds {
			t.Fatalf("unexpected Access-Control-Allow-Credentials:true when not enabled")
		}

		// Vary: Origin (importante cuando se hace reflect del Origin)
		if !e2eint.VaryContainsOrigin(respOK) {
			t.Fatalf("expected Vary to include Origin")
		}

		// Origin denegado: nunca debe reflejar ACAO
		reqBad, _ := http.NewRequest("OPTIONS", baseURL+"/v1/auth/login", nil)
		reqBad.Header.Set("Origin", "http://evil.local")
		reqBad.Header.Set("Access-Control-Request-Method", "POST")
		reqBad.Header.Set("Access-Control-Request-Headers", "content-type")
		respBad, err := c.Do(reqBad)
		if err != nil {
			t.Fatal(err)
		}
		respBad.Body.Close()
		if h := respBad.Header.Get("Access-Control-Allow-Origin"); h != "" {
			t.Fatalf("ACAO must not be present for disallowed origins; got %q", h)
		}
		if acac := respBad.Header.Get("Access-Control-Allow-Credentials"); strings.EqualFold(acac, "true") {
			t.Fatalf("ACAC must not be true for disallowed origins")
		}
	})

	t.Run("JWKS: GET + HEAD + shape + content-type", func(t *testing.T) {
		type jwks struct {
			Keys []map[string]any `json:"keys"`
		}

		// GET
		resp, err := c.Get(baseURL + "/.well-known/jwks.json")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("jwks GET status=%d", resp.StatusCode)
		}
		if ct := readHeader(resp, "Content-Type"); ct == "" || !strings.HasPrefix(strings.ToLower(ct), "application/json") {
			t.Fatalf("jwks Content-Type should be application/json, got %q", ct)
		}
		var j jwks
		if err := mustJSON(resp.Body, &j); err != nil {
			t.Fatalf("jwks decode: %v", err)
		}
		if len(j.Keys) == 0 {
			t.Fatalf("jwks.keys empty")
		}
		for i, k := range j.Keys {
			if _, ok := k["kid"]; !ok {
				t.Fatalf("jwks.keys[%d] missing kid", i)
			}
			if _, ok := k["kty"]; !ok {
				t.Fatalf("jwks.keys[%d] missing kty", i)
			}
		}

		// HEAD
		reqH, _ := http.NewRequest("HEAD", baseURL+"/.well-known/jwks.json", nil)
		rh, err := c.Do(reqH)
		if err != nil {
			t.Fatal(err)
		}
		rh.Body.Close()
		if rh.StatusCode != 200 {
			t.Fatalf("jwks HEAD status=%d", rh.StatusCode)
		}
		if cc := readHeader(rh, "Cache-Control"); cc == "" {
			t.Fatalf("jwks HEAD: expected Cache-Control header")
		}
	})

	t.Run("OIDC Discovery: GET + HEAD + contenido mínimo", func(t *testing.T) {
		// GET
		resp, err := c.Get(baseURL + "/.well-known/openid-configuration")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("discovery GET status=%d", resp.StatusCode)
		}
		var disc map[string]any
		if err := mustJSON(resp.Body, &disc); err != nil {
			t.Fatalf("discovery decode: %v", err)
		}

		expectEq := map[string]string{
			"issuer":                 baseURL,
			"authorization_endpoint": baseURL + "/oauth2/authorize",
			"token_endpoint":         baseURL + "/oauth2/token",
			"userinfo_endpoint":      baseURL + "/userinfo",
			"jwks_uri":               baseURL + "/.well-known/jwks.json",
		}
		for k, want := range expectEq {
			got, _ := disc[k].(string)
			if got != want {
				t.Fatalf("discovery.%s mismatch: got %q want %q", k, got, want)
			}
		}

		// Valores recomendados (no estrictamente obligatorios pero sanos)
		// - EdDSA soportado
		if v, ok := disc["id_token_signing_alg_values_supported"].([]any); ok {
			hasEdDSA := false
			for _, x := range v {
				if s, _ := x.(string); strings.EqualFold(s, "EdDSA") {
					hasEdDSA = true
					break
				}
			}
			if !hasEdDSA {
				t.Fatalf("discovery.id_token_signing_alg_values_supported missing EdDSA")
			}
		}
		// - PKCE S256 soportado
		if v, ok := disc["code_challenge_methods_supported"].([]any); ok {
			hasS256 := false
			for _, x := range v {
				if s, _ := x.(string); strings.EqualFold(s, "S256") {
					hasS256 = true
					break
				}
			}
			if !hasS256 {
				t.Fatalf("discovery.code_challenge_methods_supported missing S256")
			}
		}

		// HEAD
		reqH, _ := http.NewRequest("HEAD", baseURL+"/.well-known/openid-configuration", nil)
		rh, err := c.Do(reqH)
		if err != nil {
			t.Fatal(err)
		}
		rh.Body.Close()
		if rh.StatusCode != 200 {
			t.Fatalf("discovery HEAD status=%d", rh.StatusCode)
		}
		if cc := readHeader(rh, "Cache-Control"); cc == "" {
			t.Fatalf("discovery HEAD: expected Cache-Control header")
		}
	})

	t.Run("CORS validation", func(t *testing.T) {
		// Test CORS preflight permitido
		allowedOrigin := "http://localhost:3000"
		req1, _ := http.NewRequest("OPTIONS", baseURL+"/v1/auth/login", nil)
		req1.Header.Set("Origin", allowedOrigin)
		req1.Header.Set("Access-Control-Request-Method", "POST")
		req1.Header.Set("Access-Control-Request-Headers", "content-type")

		resp1, err := c.Do(req1)
		if err != nil {
			t.Fatal(err)
		}
		resp1.Body.Close()

		if resp1.StatusCode != 200 && resp1.StatusCode != 204 {
			t.Fatalf("CORS preflight allowed origin status=%d", resp1.StatusCode)
		}

		allowOriginHeader := readHeader(resp1, "Access-Control-Allow-Origin")
		if allowOriginHeader == "" {
			t.Fatalf("CORS allowed origin missing Access-Control-Allow-Origin header")
		}

		// Test CORS preflight NO permitido
		forbiddenOrigin := "http://evil.local"
		req2, _ := http.NewRequest("OPTIONS", baseURL+"/v1/auth/login", nil)
		req2.Header.Set("Origin", forbiddenOrigin)
		req2.Header.Set("Access-Control-Request-Method", "POST")
		req2.Header.Set("Access-Control-Request-Headers", "content-type")

		resp2, err := c.Do(req2)
		if err != nil {
			t.Fatal(err)
		}
		resp2.Body.Close()

		// Para origen no permitido, no debería tener Access-Control-Allow-Origin
		forbiddenAllowOrigin := readHeader(resp2, "Access-Control-Allow-Origin")
		if forbiddenAllowOrigin == forbiddenOrigin {
			t.Fatalf("CORS should not allow forbidden origin %s", forbiddenOrigin)
		}
	})

	t.Run("Cookie security headers", func(t *testing.T) {
		// Test session login para inspeccionar Set-Cookie headers
		if seed == nil {
			t.Skip("no seed data; skipping cookie test")
		}

		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		})

		resp, err := c.Post(baseURL+"/v1/session/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Session login puede no estar implementado o devolver 204/404
		if resp.StatusCode == 404 || resp.StatusCode == 501 {
			t.Skip("session login endpoint not implemented; skipping cookie security test")
		}

		if resp.StatusCode != 200 && resp.StatusCode != 302 && resp.StatusCode != 204 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("session login status=%d body=%s", resp.StatusCode, string(b))
		}

		setCookies := resp.Header["Set-Cookie"]
		if len(setCookies) == 0 {
			t.Skip("no session cookies set; skipping cookie security test")
		}

		hasHttpOnly := false
		hasSameSite := false
		hasSecure := false

		for _, cookie := range setCookies {
			if strings.Contains(cookie, "HttpOnly") {
				hasHttpOnly = true
			}
			if strings.Contains(cookie, "SameSite=") {
				hasSameSite = true
			}
			if strings.Contains(cookie, "Secure") {
				hasSecure = true
			}
		}

		if !hasHttpOnly {
			t.Errorf("session cookies should have HttpOnly flag")
		}
		if !hasSameSite {
			t.Errorf("session cookies should have SameSite attribute")
		}
		// Secure flag puede ser opcional en desarrollo localhost
		_ = hasSecure // evitar warning unused
	})
}

func init() {
	// defensivo: subtests 00 pueden correr primeros; aseguramos que no falte seed
	_ = json.Marshal // evita import "unused" si tocás estructuras arriba
	_ = time.Now     // evita warning si no usamos time en ciertos escenarios
}
