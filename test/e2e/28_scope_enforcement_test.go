package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// 28 - Enforcement de scopes en recursos (profile:read)
func Test_28_Scope_Enforcement(t *testing.T) {
	if seed == nil {
		t.Skip("no seed")
	}

	// 1) Obtener access token sin profile:read (solo openid email profile por default)
	c := newHTTPClient()
	body, _ := json.Marshal(map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	})
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("auth login status=%d body=%s", resp.StatusCode, string(b))
	}
	var tk struct {
		AccessToken string `json:"access_token"`
	}
	if err := mustJSON(resp.Body, &tk); err != nil {
		t.Fatal(err)
	}

	// 1.a) Llamar /v1/profile con token sin scope -> 403
	req, _ := http.NewRequest("GET", baseURL+"/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tk.AccessToken)
	r1, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r1.Body)
	r1.Body.Close()
	if r1.StatusCode != 403 {
		t.Fatalf("expected 403 without profile:read, got %d", r1.StatusCode)
	}

	// 2) Fluir OIDC con consentimiento de profile:read y pedir access con ese scope
	// Primero establecer cookie de sesión
	resp2, err := c.Post(baseURL+"/v1/session/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 204 {
		t.Fatalf("session login=%d", resp2.StatusCode)
	}

	// Pedimos authorize con scope openid profile:read
	redirect := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirect = seed.Clients.Web.Redirects[0]
	}
	verifier := newCodeVerifier()
	challenge := pkceS256(verifier)
	authQ := url.Values{}
	authQ.Set("response_type", "code")
	authQ.Set("client_id", seed.Clients.Web.ClientID)
	authQ.Set("redirect_uri", redirect)
	authQ.Set("scope", "openid profile:read")
	authQ.Set("state", "s28")
	authQ.Set("code_challenge", challenge)
	authQ.Set("code_challenge_method", "S256")

	clNoFollow := newHTTPClient()
	clNoFollow.Jar = c.Jar
	clNoFollow.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	rAuth, err := clNoFollow.Get(baseURL + "/oauth2/authorize?" + authQ.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer rAuth.Body.Close()
	if rAuth.StatusCode == 200 {
		// consent_required path
		var msg struct {
			ConsentRequired bool   `json:"consent_required"`
			ConsentToken    string `json:"consent_token"`
		}
		if err := mustJSON(rAuth.Body, &msg); err != nil {
			t.Fatal(err)
		}
		if !msg.ConsentRequired || msg.ConsentToken == "" {
			t.Fatalf("expected consent_required with token")
		}
		// aceptar consentimiento
		b, _ := json.Marshal(map[string]any{"consent_token": msg.ConsentToken, "approve": true})
		// Use no-follow client to avoid trying to reach the frontend redirect_uri (localhost:3000)
		clNoFollow2 := newHTTPClient()
		clNoFollow2.Jar = c.Jar
		clNoFollow2.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
		rc, err := clNoFollow2.Post(baseURL+"/v1/auth/consent/accept", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Body.Close()
		if rc.StatusCode != 302 {
			t.Fatalf("consent accept expected 302, got %d", rc.StatusCode)
		}
		loc := readHeader(rc, "Location")
		if loc == "" {
			t.Fatalf("missing Location after consent")
		}
		code := qs(loc, "code")
		if code == "" {
			t.Fatalf("missing code after consent redirect")
		}

		// token exchange
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("code_verifier", verifier)
		rtok, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		defer rtok.Body.Close()
		if rtok.StatusCode != 200 {
			t.Fatalf("token exchange=%d", rtok.StatusCode)
		}
		var out struct {
			AccessToken string `json:"access_token"`
		}
		if err := mustJSON(rtok.Body, &out); err != nil {
			t.Fatal(err)
		}

		// 2.a) now /v1/profile should be 200
		rq, _ := http.NewRequest("GET", baseURL+"/v1/profile", nil)
		rq.Header.Set("Authorization", "Bearer "+out.AccessToken)
		r2, err := c.Do(rq)
		if err != nil {
			t.Fatal(err)
		}
		// header sanity
		if ct := readHeader(r2, "Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
			t.Fatalf("bad content-type: %s", ct)
		}
		if cc := readHeader(r2, "Cache-Control"); !strings.Contains(strings.ToLower(cc), "no-store") {
			t.Fatalf("missing no-store: %s", cc)
		}
		// body shape sanity
		var prof map[string]any
		if err := mustJSON(r2.Body, &prof); err != nil {
			t.Fatal(err)
		}
		r2.Body.Close()
		if as := asString(prof["sub"]); as == "" {
			t.Fatalf("sub empty")
		}
		if as := asString(prof["email"]); as == "" {
			t.Fatalf("email empty")
		}
		if _, ok := prof["email_verified"].(bool); !ok {
			t.Fatalf("email_verified not bool")
		}
		switch v := prof["updated_at"].(type) {
		case float64, int64, int32, int:
			// ok
		default:
			t.Fatalf("updated_at not numeric: %T", v)
		}
		return
	}

	// En caso de autoconsent habilitado y redirect 302 directo: seguir flujo normal
	if rAuth.StatusCode != 302 {
		b, _ := io.ReadAll(rAuth.Body)
		t.Fatalf("authorize expected 302 or consent 200, got %d body=%s", rAuth.StatusCode, string(b))
	}
	loc := readHeader(rAuth, "Location")
	code := qs(loc, "code")
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("client_id", seed.Clients.Web.ClientID)
	form.Set("code_verifier", verifier)
	rtok, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer rtok.Body.Close()
	if rtok.StatusCode != 200 {
		t.Fatalf("token exchange=%d", rtok.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := mustJSON(rtok.Body, &out); err != nil {
		t.Fatal(err)
	}

	rq, _ := http.NewRequest("GET", baseURL+"/v1/profile", nil)
	rq.Header.Set("Authorization", "Bearer "+out.AccessToken)
	r2, err := c.Do(rq)
	if err != nil {
		t.Fatal(err)
	}
	if r2.StatusCode != 200 {
		t.Fatalf("expected 200 with profile:read, got %d", r2.StatusCode)
	}
	if ct := readHeader(r2, "Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		t.Fatalf("bad content-type: %s", ct)
	}
	if cc := readHeader(r2, "Cache-Control"); !strings.Contains(strings.ToLower(cc), "no-store") {
		t.Fatalf("missing no-store: %s", cc)
	}
	var prof map[string]any
	if err := mustJSON(r2.Body, &prof); err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if as := asString(prof["sub"]); as == "" {
		t.Fatalf("sub empty")
	}
	if as := asString(prof["email"]); as == "" {
		t.Fatalf("email empty")
	}
	if _, ok := prof["email_verified"].(bool); !ok {
		t.Fatalf("email_verified not bool")
	}
	switch v := prof["updated_at"].(type) {
	case float64, int64, int32, int:
		// ok
	default:
		t.Fatalf("updated_at not numeric: %T", v)
	}
}
