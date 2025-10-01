package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	jwti "github.com/dropDatabas3/hellojohn/test/e2e/internal"
)

// 01 - Auth básica con password: register/login, /v1/me, refresh con rotación,
// logout, userinfo y validaciones mínimas del JWT emitido.
func Test_01_Auth_Basic(t *testing.T) {
	c := newHTTPClient()

	// Construimos un email único reutilizando el dominio de admin (plus addressing)
	email := uniqueEmail(seed.Users.Admin.Email, "e2e01")

	// Passwords para el flujo
	const initialPwd = "SuperSecreta1!"
	// newPwd no se usa acá (el reset está en otros subtests), lo mantenemos simple.

	type regReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type loginReq = regReq
	type tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	var access, refresh string

	t.Run("register (o idempotente) -> tokens o 200/201", func(t *testing.T) {
		body, _ := json.Marshal(regReq{
			TenantID: seed.Tenant.ID,
			ClientID: seed.Clients.Web.ClientID,
			Email:    email,
			Password: initialPwd,
		})
		resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("register status=%d body=%s", resp.StatusCode, string(b))
		}

		var out tokens
		_ = mustJSON(resp.Body, &out) // puede no devolver cuerpo JSON (aceptamos 204/201 sin body)
		// Si devolvió tokens, los aprovechamos. Caso contrario, logueamos en el paso siguiente.
		access = out.AccessToken
		refresh = out.RefreshToken
	})

	t.Run("login (si registro no devolvió tokens)", func(t *testing.T) {
		if access != "" && refresh != "" {
			t.Skip("register already returned tokens")
		}
		body, _ := json.Marshal(loginReq{
			TenantID: seed.Tenant.ID,
			ClientID: seed.Clients.Web.ClientID,
			Email:    email,
			Password: initialPwd,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("login status=%d body=%s", resp.StatusCode, string(b))
		}
		var out tokens
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.AccessToken == "" || out.RefreshToken == "" {
			t.Fatalf("login missing tokens: %+v", out)
		}
		access = out.AccessToken
		refresh = out.RefreshToken
	})

	t.Run("/v1/me (Bearer) OK", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("/v1/me status=%d", resp.StatusCode)
		}
		var me map[string]any
		if err := mustJSON(resp.Body, &me); err != nil {
			t.Fatal(err)
		}
		// checks mínimos
		if me["sub"] == nil {
			t.Fatalf("/v1/me missing sub")
		}
		if me["tid"] != seed.Tenant.ID {
			t.Fatalf("/v1/me tid mismatch: got=%v want=%s", me["tid"], seed.Tenant.ID)
		}
	})

	t.Run("/userinfo (Bearer) OK", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("/userinfo status=%d", resp.StatusCode)
		}
	})

	t.Run("JWT structure & claims principales", func(t *testing.T) {
		hdr, pld, err := decodeJWT(access)
		if err != nil {
			t.Fatalf("decode access token: %v", err)
		}

		// Header.alg recomendado: EdDSA (Ed25519)
		if alg, _ := hdr["alg"].(string); !strings.EqualFold(alg, "EdDSA") {
			t.Fatalf("unexpected alg=%v (expected EdDSA)", hdr["alg"])
		}
		// aud == client_id
		if aud, _ := pld["aud"].(string); aud != seed.Clients.Web.ClientID {
			t.Fatalf("aud mismatch: got=%v want=%s", pld["aud"], seed.Clients.Web.ClientID)
		}
		// tid == tenant_id
		if tid, _ := pld["tid"].(string); tid != seed.Tenant.ID {
			t.Fatalf("tid mismatch: got=%v want=%s", pld["tid"], seed.Tenant.ID)
		}
		// iss == baseURL
		if iss, _ := pld["iss"].(string); iss != baseURL {
			t.Fatalf("iss mismatch: got=%v want=%s", pld["iss"], baseURL)
		}
		// exp > now
		switch exp := pld["exp"].(type) {
		case float64:
			if int64(exp) <= time.Now().Unix() {
				t.Fatalf("token expired: exp=%v now=%v", exp, time.Now().Unix())
			}
		default:
			t.Fatalf("exp not present/number: %v", pld["exp"])
		}
	})

	t.Run("refresh (rotación) + no-store/no-cache headers", func(t *testing.T) {
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(refreshReq{
			ClientID:     seed.Clients.Web.ClientID,
			RefreshToken: refresh,
		})
		resp, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("refresh status=%d body=%s", resp.StatusCode, string(b))
		}
		// headers de seguridad en /token-like endpoints
		if cc := readHeader(resp, "Cache-Control"); !strings.EqualFold(cc, "no-store") {
			t.Fatalf("expected Cache-Control=no-store; got %q", cc)
		}
		if pg := readHeader(resp, "Pragma"); !strings.EqualFold(pg, "no-cache") {
			t.Fatalf("expected Pragma=no-cache; got %q", pg)
		}

		var out tokens
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.RefreshToken == "" || out.AccessToken == "" {
			t.Fatalf("refresh missing tokens: %+v", out)
		}
		if out.RefreshToken == refresh {
			t.Fatalf("refresh token did not rotate")
		}

		// Reuso del refresh viejo -> debe fallar (401)
		old := refresh
		refresh = out.RefreshToken

		bodyOld, _ := json.Marshal(refreshReq{
			ClientID:     seed.Clients.Web.ClientID,
			RefreshToken: old,
		})
		r2, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(bodyOld))
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()
		if r2.StatusCode != 401 {
			b, _ := io.ReadAll(r2.Body)
			t.Fatalf("expected 401 when reusing old refresh; got %d body=%s", r2.StatusCode, string(b))
		}
	})

	t.Run("logout revoca refresh actual", func(t *testing.T) {
		type logoutReq struct {
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(logoutReq{RefreshToken: refresh})
		resp, err := c.Post(baseURL+"/v1/auth/logout", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			t.Fatalf("logout status=%d", resp.StatusCode)
		}

		// Intento de refresh con el token revocado -> 401
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		body2, _ := json.Marshal(refreshReq{
			ClientID:     seed.Clients.Web.ClientID,
			RefreshToken: refresh,
		})
		r2, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body2))
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()
		if r2.StatusCode != 401 {
			t.Fatalf("expected 401 after logout; got %d", r2.StatusCode)
		}
	})

	t.Run("login inválido (wrong password) -> 401", func(t *testing.T) {
		body, _ := json.Marshal(loginReq{
			TenantID: seed.Tenant.ID,
			ClientID: seed.Clients.Web.ClientID,
			Email:    email,
			Password: "WrongPass!!!",
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401 with wrong credentials; got %d", resp.StatusCode)
		}
	})

	// Verificación criptográfica real (firma + tiempos) contra JWKS
	t.Run("JWT cryptographic verification (JWKS)", func(t *testing.T) {
		claims, _, err := jwti.VerifyJWTWithJWKS(context.Background(), baseURL, access, baseURL, seed.Clients.Web.ClientID, 60*time.Second)
		if err != nil {
			t.Fatalf("verify access with jwks: %v", err)
		}
		// sanity: same tid/aud
		if aud, _ := claims["aud"].(string); aud != "" && aud != seed.Clients.Web.ClientID {
			t.Fatalf("aud mismatch after verify: %v", claims["aud"])
		}
	})

	// Negativo: forzar kid inexistente debe fallar
	t.Run("JWT with unknown kid fails", func(t *testing.T) {
		parts := strings.Split(access, ".")
		if len(parts) < 3 {
			t.Skip("malformed token in env")
		}
		// header -> set fake kid
		var hdr map[string]any
		_ = json.Unmarshal(mustB64(parts[0]), &hdr)
		hdr["kid"] = "no-such-kid-e2e"
		b, _ := json.Marshal(hdr)
		fakeHeader := b64url(b)
		mut := fakeHeader + "." + parts[1] + "." + parts[2]
		_, _, err := jwti.VerifyJWTWithJWKS(context.Background(), baseURL, mut, baseURL, seed.Clients.Web.ClientID, 60*time.Second)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "kid") {
			t.Fatalf("expected kid not found error, got: %v", err)
		}
	})
}

// helpers
func mustB64(seg string) []byte {
	// base64url decode
	s := strings.ReplaceAll(seg, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	b, _ := base64.StdEncoding.DecodeString(s)
	return b
}
