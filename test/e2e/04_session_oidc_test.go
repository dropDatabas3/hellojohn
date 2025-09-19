package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// 04 - OIDC con sesión por cookie (Code + PKCE) + userinfo + refresh + revoke.
// Usa /v1/session/login para establecer la cookie y luego /oauth2/* para el flujo.
func Test_04_Session_OIDC_Code_PKCE(t *testing.T) {
	c := newHTTPClient() // tiene CookieJar

	// 1) Session login (set-cookie)
	{
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
		if resp.StatusCode != 204 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("session login status=%d body=%s", resp.StatusCode, string(b))
		}
		// (opcional) sanity headers de cookie
		if sc := readHeader(resp, "Set-Cookie"); sc == "" {
			t.Fatalf("no Set-Cookie on session/login")
		}
	}

	// redirect_uri del cliente (fallback dev si falta)
	redirect := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirect = seed.Clients.Web.Redirects[0]
	}

	// 2) /oauth2/authorize con cookie -> 302 con code&state
	noFollow := func() *http.Client {
		cl := newHTTPClient()
		cl.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		// copiamos cookies del cliente principal
		cl.Jar = c.Jar
		return cl
	}()

	verifier := newCodeVerifier()
	challenge := pkceS256(verifier)
	state := fmt.Sprintf("st%d", time.Now().UnixNano())
	nonce := fmt.Sprintf("nn%d", time.Now().UnixNano())

	authQ := url.Values{}
	authQ.Set("response_type", "code")
	authQ.Set("client_id", seed.Clients.Web.ClientID)
	authQ.Set("redirect_uri", redirect)
	authQ.Set("scope", "openid email profile")
	authQ.Set("state", state)
	authQ.Set("nonce", nonce)
	authQ.Set("code_challenge", challenge)
	authQ.Set("code_challenge_method", "S256")

	authURL := baseURL + "/oauth2/authorize?" + authQ.Encode()

	rAuth, err := noFollow.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	defer rAuth.Body.Close()
	if rAuth.StatusCode != 302 {
		b, _ := io.ReadAll(rAuth.Body)
		t.Fatalf("authorize expected 302, got %d body=%s", rAuth.StatusCode, string(b))
	}
	loc := readHeader(rAuth, "Location")
	if loc == "" {
		t.Fatalf("authorize missing Location")
	}
	code := qs(loc, "code")
	stRet := qs(loc, "state")
	if code == "" {
		t.Fatalf("authorize missing ?code in redirect")
	}
	if stRet != state {
		t.Fatalf("state mismatch: want=%s got=%s", state, stRet)
	}

	// 3) /oauth2/token (auth_code + PKCE)
	var at, rt, idt string
	{
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("code_verifier", verifier)

		resp, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("token (auth_code) status=%d body=%s", resp.StatusCode, string(b))
		}
		// headers de seguridad
		if cc := readHeader(resp, "Cache-Control"); !strings.EqualFold(cc, "no-store") {
			t.Fatalf("token Cache-Control expected no-store, got %q", cc)
		}
		if pg := readHeader(resp, "Pragma"); !strings.EqualFold(pg, "no-cache") {
			t.Fatalf("token Pragma expected no-cache, got %q", pg)
		}

		var tok struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			IDToken      string `json:"id_token"`
			TokenType    string `json:"token_type"`
			Scope        string `json:"scope"`
			ExpiresIn    int64  `json:"expires_in"`
		}
		if err := mustJSON(resp.Body, &tok); err != nil {
			t.Fatal(err)
		}
		if tok.TokenType != "Bearer" || tok.AccessToken == "" || tok.RefreshToken == "" || tok.IDToken == "" || tok.ExpiresIn <= 0 {
			t.Fatalf("invalid token response: %+v", tok)
		}
		at, rt, idt = tok.AccessToken, tok.RefreshToken, tok.IDToken

		// Validar claims básicos del ID Token
		hdr, pld := jwtHeaderPayload(idt)
		if hdr == nil || pld == nil {
			t.Fatalf("id_token not decodable")
		}
		// azp y tid (si vienen)
		if s := asString(pld["azp"]); s != "" && s != seed.Clients.Web.ClientID {
			t.Fatalf("id_token.azp mismatch; got %s", s)
		}
		if s := asString(pld["tid"]); s != "" && s != seed.Tenant.ID {
			t.Fatalf("id_token.tid mismatch; got %s", s)
		}
		// at_hash (si viene)
		if ah := asString(pld["at_hash"]); ah != "" {
			if ah != atHash(at) {
				t.Fatalf("id_token.at_hash mismatch")
			}
		}
		// nonce (si viene)
		if n := asString(pld["nonce"]); n != "" && n != nonce {
			t.Fatalf("id_token.nonce mismatch")
		}
	}

	// 4) /userinfo con Bearer
	{
		req, _ := http.NewRequest("GET", baseURL+"/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+at)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("userinfo status=%d body=%s", resp.StatusCode, string(b))
		}
	}

	// 5) refresh grant (rotación)
	var rt2 string
	{
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("refresh_token", rt)

		resp, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("token (refresh) status=%d body=%s", resp.StatusCode, string(b))
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
			TokenType    string `json:"token_type"`
			Scope        string `json:"scope"`
		}
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.RefreshToken == "" || out.RefreshToken == rt {
			t.Fatalf("refresh must rotate")
		}
		rt2 = out.RefreshToken
	}

	// reuse del refresh viejo → invalid_grant (400) o 401, según implementación
	{
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("refresh_token", rt) // viejo
		resp, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 && resp.StatusCode != 401 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400/401 on old refresh, got %d body=%s", resp.StatusCode, string(b))
		}
	}

	// 6) revoke + refresh debe fallar
	{
		form := url.Values{}
		form.Set("token", rt2)
		form.Set("token_type_hint", "refresh_token")
		resp, err := c.Post(baseURL+"/oauth2/revoke", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("revoke status=%d", resp.StatusCode)
		}

		form = url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("refresh_token", rt2)
		r2, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()
		if r2.StatusCode != 400 && r2.StatusCode != 401 {
			t.Fatalf("expected invalid_grant after revoke; got %d", r2.StatusCode)
		}
	}
}
