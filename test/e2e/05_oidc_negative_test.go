package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// 05 - OIDC negativos / hardening:
// - login_required sin sesión
// - invalid_scope con state preservado
// - sin openid => 400
// - PKCE inválido => invalid_grant
// - reuso de authorization code => invalid_grant
// - redirect_uri distinto => invalid_grant
// - token unsupported_grant_type => 400
// - userinfo sin bearer => 401
func Test_05_OIDC_Negative(t *testing.T) {
	// redirect del cliente
	redirect := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirect = seed.Clients.Web.Redirects[0]
	}

	noFollow := func() *http.Client {
		cl := newHTTPClient()
		cl.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		return cl
	}()

	// --- login_required: authorize sin sesión
	t.Run("login_required without session", func(t *testing.T) {
		state := "lr" + itoa(time.Now().Unix()%1_000_000)
		verifier := "abc" + itoa(time.Now().Unix()%1_000_000)
		challenge := pkceS256(verifier)

		q := url.Values{}
		q.Set("response_type", "code")
		q.Set("client_id", seed.Clients.Web.ClientID)
		q.Set("redirect_uri", redirect)
		q.Set("scope", "openid email")
		q.Set("state", state)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")

		resp, err := noFollow.Get(baseURL + "/oauth2/authorize?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// La mayoría de implementaciones redirige con error=login_required (&state)
		// pero aceptamos variantes (302 o 401 en estrictos).
		if resp.StatusCode == 302 {
			loc := readHeader(resp, "Location")
			if !strings.Contains(loc, "error=login_required") || !strings.Contains(loc, "state="+state) {
				t.Fatalf("expected login_required with state preserved (Location=%q)", loc)
			}
		} else if resp.StatusCode != 401 {
			t.Fatalf("expected 302 (login_required) or 401; got %d", resp.StatusCode)
		}
	})

	// --- invalid_scope con state preservado
	t.Run("invalid_scope preserves state", func(t *testing.T) {
		state := "stX" + itoa(time.Now().Unix()%1_000_000)
		verifier := "def" + itoa(time.Now().Unix()%1_000_000)
		challenge := pkceS256(verifier)

		q := url.Values{}
		q.Set("response_type", "code")
		q.Set("client_id", seed.Clients.Web.ClientID)
		q.Set("redirect_uri", redirect)
		q.Set("scope", "openid admin") // 'admin' supuesto no permitido
		q.Set("state", state)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")

		resp, err := noFollow.Get(baseURL + "/oauth2/authorize?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 302 {
			t.Fatalf("expected 302 (invalid_scope), got %d", resp.StatusCode)
		}
		loc := readHeader(resp, "Location")
		if !strings.Contains(loc, "error=invalid_scope") || !strings.Contains(loc, "state="+state) {
			t.Fatalf("expected invalid_scope with state preserved (Location=%q)", loc)
		}
	})

	// --- sin openid => 400 invalid_request/invalid_scope
	t.Run("missing openid scope -> 400", func(t *testing.T) {
		verifier := "ghi" + itoa(time.Now().Unix()%1_000_000)
		challenge := pkceS256(verifier)

		q := url.Values{}
		q.Set("response_type", "code")
		q.Set("client_id", seed.Clients.Web.ClientID)
		q.Set("redirect_uri", redirect)
		q.Set("scope", "email profile") // sin openid
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")

		resp, err := newHTTPClient().Get(baseURL + "/oauth2/authorize?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("expected 400 without openid, got %d", resp.StatusCode)
		}
	})

	// Preparamos una sesión + authorization code válido para testear PKCE/redirect reuse.
	cSess := newHTTPClient()
	{
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		})
		resp, err := cSess.Post(baseURL+"/v1/session/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 {
			t.Fatalf("session login status=%d", resp.StatusCode)
		}
	}

	// helper authorize (devuelve code)
	getCode := func(verifier string) (string, string) {
		q := url.Values{}
		q.Set("response_type", "code")
		q.Set("client_id", seed.Clients.Web.ClientID)
		q.Set("redirect_uri", redirect)
		q.Set("scope", "openid email")
		st := "st" + itoa(time.Now().Unix()%1_000_000)
		q.Set("state", st)
		q.Set("code_challenge", pkceS256(verifier))
		q.Set("code_challenge_method", "S256")

		cl := newHTTPClient()
		cl.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		cl.Jar = cSess.Jar // copia cookies

		resp, err := cl.Get(baseURL + "/oauth2/authorize?" + q.Encode())
		if err != nil {
			return "", st
		}
		defer resp.Body.Close()
		loc := readHeader(resp, "Location")
		return qs(loc, "code"), st
	}

	// --- PKCE inválido (code_verifier incorrecto) => invalid_grant
	t.Run("invalid PKCE -> invalid_grant", func(t *testing.T) {
		verifier := "good-" + itoa(time.Now().Unix()%1_000_000)
		code, _ := getCode(verifier)
		if code == "" {
			t.Fatalf("failed to get authorization code for PKCE test")
		}

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("code_verifier", "wrong-verifier")

		resp, err := newHTTPClient().Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("expected 400 invalid_grant (pkce), got %d", resp.StatusCode)
		}
	})

	// --- Reuso de authorization code => invalid_grant
	t.Run("reuse auth code -> invalid_grant", func(t *testing.T) {
		verifier := "ok-" + itoa(time.Now().Unix()%1_000_000)
		code, _ := getCode(verifier)
		if code == "" {
			t.Fatalf("failed to get authorization code")
		}

		// canje válido
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("code_verifier", verifier)

		resp1, err := newHTTPClient().Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp1.Body)
		resp1.Body.Close()
		if resp1.StatusCode != 200 {
			t.Fatalf("first token exchange status=%d", resp1.StatusCode)
		}

		// reuso => 400 invalid_grant
		resp2, err := newHTTPClient().Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != 400 {
			t.Fatalf("expected 400 on code reuse, got %d", resp2.StatusCode)
		}
	})

	// --- redirect_uri distinto => invalid_grant
	t.Run("redirect_uri mismatch -> invalid_grant", func(t *testing.T) {
		verifier := "ok2-" + itoa(time.Now().Unix()%1_000_000)
		code, _ := getCode(verifier)
		if code == "" {
			t.Fatalf("failed to get authorization code")
		}

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", "http://localhost:3000/other") // NO permitido
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("code_verifier", verifier)

		resp, err := newHTTPClient().Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("expected 400 on redirect_uri mismatch, got %d", resp.StatusCode)
		}
	})

	// --- token endpoint: unsupported_grant_type => 400
	t.Run("unsupported_grant_type -> 400", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "foobar")
		resp, err := newHTTPClient().Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("expected 400 on unsupported_grant_type, got %d", resp.StatusCode)
		}
	})

	// --- userinfo sin bearer => 401
	t.Run("userinfo without bearer -> 401", func(t *testing.T) {
		resp, err := newHTTPClient().Get(baseURL + "/userinfo")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}
