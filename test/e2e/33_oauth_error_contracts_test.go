package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// 33 - OAuth/OIDC error contracts per RFC6750/6749
func Test_33_OAuth_Error_Contracts(t *testing.T) {
	c := newHTTPClient()

	// Missing token -> 401 + WWW-Authenticate: Bearer error="invalid_token"
	req, _ := http.NewRequest("GET", baseURL+"/userinfo", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for missing token, got %d", resp.StatusCode)
	}
	wa := resp.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(strings.ToLower(wa), "bearer") || !strings.Contains(strings.ToLower(wa), "invalid_token") {
		t.Fatalf("WWW-Authenticate should indicate Bearer invalid_token, got %q", wa)
	}

	// Insufficient scope: request /v1/profile without scope
	// Acquire tokens with openid only
	at := obtainAccessTokenWithScopes(t, "openid")
	if at == "" {
		t.Skip("cannot obtain openid-only access token")
	}
	r2, _ := http.NewRequest("GET", baseURL+"/v1/profile", nil)
	r2.Header.Set("Authorization", "Bearer "+at)
	resp2, err := c.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 403 {
		t.Fatalf("expected 403 insufficient_scope, got %d", resp2.StatusCode)
	}
	wa2 := resp2.Header.Get("WWW-Authenticate")
	if !strings.Contains(strings.ToLower(wa2), "insufficient_scope") {
		t.Fatalf("expected insufficient_scope in WWW-Authenticate, got %q", wa2)
	}
}

// obtainAccessTokenWithScopes performs a minimal code flow to issue an access token with provided scopes, consenting automatically.
func obtainAccessTokenWithScopes(t *testing.T, scopes string) string {
	t.Helper()
	c := newHTTPClient()
	// Session login first
	body := map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	}
	b, _ := jsonMarshal(body)
	resp, err := c.Post(baseURL+"/v1/session/login", "application/json", strings.NewReader(string(b)))
	if err != nil || (resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 302) {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	if resp != nil {
		resp.Body.Close()
	}

	// No-follow client with copied cookies
	nf := *c
	nf.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	nf.Jar = c.Jar
	redirect := "http://localhost:3000/callback"
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", seed.Clients.Web.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("scope", scopes)
	q.Set("state", "s3")
	q.Set("nonce", "n3")
	q.Set("code_challenge_method", "S256")
	q.Set("code_challenge", pkceS256(newCodeVerifier()))
	rr, err := nf.Get(baseURL + "/oauth2/authorize?" + q.Encode())
	if err != nil || rr.StatusCode != 302 {
		if rr != nil {
			rr.Body.Close()
		}
		return ""
	}
	loc := readHeader(rr, "Location")
	rr.Body.Close()
	if loc == "" {
		return ""
	}
	code := qs(loc, "code")
	if code == "" {
		return ""
	}

	// Exchange for tokens
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", seed.Clients.Web.ClientID)
	form.Set("redirect_uri", redirect)
	form.Set("code_verifier", newCodeVerifier())
	tokResp, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil || tokResp.StatusCode != 200 {
		if tokResp != nil {
			tokResp.Body.Close()
		}
		return ""
	}
	defer tokResp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(tokResp.Body).Decode(&tok)
	return tok.AccessToken
}
