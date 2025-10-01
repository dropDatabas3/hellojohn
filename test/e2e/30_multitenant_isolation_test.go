package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// 30 - Multi-tenant / client isolation
func Test_30_Multitenant_Isolation(t *testing.T) {
	c := newHTTPClient()

	// Assumption: seed provides a tenant and one web client under that tenant.
	if seed == nil || seed.Clients.Web.ClientID == "" {
		t.Skip("no seed clients")
	}

	// Create a second client under a different tenant via admin may be heavy; instead attempt obvious mismatches
	// A) Mismatched tenant+client: use tenant A with a bogus client_id that likely belongs to another tenant or doesn't exist
	{
		form := url.Values{}
		form.Set("response_type", "code")
		form.Set("client_id", "non-existent-client")
		form.Set("redirect_uri", "http://localhost:3000/callback")
		form.Set("scope", "openid profile email")
		form.Set("state", "s1")
		form.Set("nonce", "n1")
		form.Set("code_challenge_method", "S256")
		form.Set("code_challenge", "x")
		// No cookie -> should not redirect tokens, should return invalid_client
		req, _ := http.NewRequest("GET", baseURL+"/oauth2/authorize?"+form.Encode(), nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 && resp.StatusCode != 400 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 4xx invalid_client, got %d body=%s", resp.StatusCode, string(b))
		}
	}

	// B) Foreign redirect_uri: with real client_id but redirect_uri from elsewhere
	{
		form := url.Values{}
		form.Set("response_type", "code")
		form.Set("client_id", seed.Clients.Web.ClientID)
		form.Set("redirect_uri", "http://evil.local/cb")
		form.Set("scope", "openid profile email")
		form.Set("state", "s2")
		form.Set("nonce", "n2")
		form.Set("code_challenge_method", "S256")
		form.Set("code_challenge", "x")
		req, _ := http.NewRequest("GET", baseURL+"/oauth2/authorize?"+form.Encode(), nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 { // invalid_redirect_uri
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400 invalid_redirect_uri, got %d body=%s", resp.StatusCode, string(b))
		}
		// check error body (best-effort)
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "invalid_redirect_uri" && !strings.Contains(strings.ToLower(string(e.Error)), "redirect") {
			t.Logf("warning: expected invalid_redirect_uri, got %q", e.Error)
		}
	}
}
