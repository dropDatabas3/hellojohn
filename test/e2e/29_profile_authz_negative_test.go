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

// 29 - Profile rejects ID token (must be access token with profile:read)
func Test_29_Profile_Rejects_ID_Token(t *testing.T) {
	if seed == nil {
		t.Skip("no seed")
	}
	c := newHTTPClient()

	// Establish cookie session first
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
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("session login=%d", resp.StatusCode)
	}

	// Perform OIDC authorize to get an ID token via code flow (scope openid only)
	redirect := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirect = seed.Clients.Web.Redirects[0]
	}
	verifier := newCodeVerifier()
	challenge := pkceS256(verifier)
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", seed.Clients.Web.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("scope", "openid") // deliberately no profile:read
	q.Set("state", "s29")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	// no-follow to capture code
	clNF := newHTTPClient()
	clNF.Jar = c.Jar
	clNF.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	ra, err := clNF.Get(baseURL + "/oauth2/authorize?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer ra.Body.Close()

	if ra.StatusCode != 302 {
		b, _ := io.ReadAll(ra.Body)
		t.Fatalf("authorize expected 302 (openid only), got %d body=%s", ra.StatusCode, string(b))
	}
	loc := readHeader(ra, "Location")
	code := qs(loc, "code")
	if code == "" {
		t.Fatalf("missing code")
	}

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
		IDToken string `json:"id_token"`
	}
	if err := mustJSON(rtok.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.IDToken == "" {
		t.Fatalf("missing id_token")
	}

	// Try using the ID token as Bearer on /v1/profile
	req, _ := http.NewRequest("GET", baseURL+"/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+out.IDToken)
	r2, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()

	// Expect 403 (insufficient_scope) or 401 (invalid_token)
	if r2.StatusCode != 403 && r2.StatusCode != 401 {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("expected 403/401 for ID token; got %d body=%s", r2.StatusCode, string(b))
	}
	// No profile fields in error body (best-effort sanity)
	b, _ := io.ReadAll(r2.Body)
	s := strings.ToLower(string(b))
	if strings.Contains(s, "\"email\"") || strings.Contains(s, "\"given_name\"") || strings.Contains(s, "\"family_name\"") {
		t.Fatalf("error body leaked profile fields: %s", string(b))
	}
}
