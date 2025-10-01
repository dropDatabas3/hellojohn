package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// 32 - Token TTL expirations under short TEST_* TTLs
func Test_32_Token_TTL(t *testing.T) {
	if os.Getenv("TEST_ACCESS_TTL") == "" || os.Getenv("TEST_REFRESH_TTL") == "" {
		t.Skip("TEST_ACCESS_TTL/TEST_REFRESH_TTL not set; skipping")
	}
	c := newHTTPClient()
	// login password to get tokens
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
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Skipf("auth/login failed: %d %s", resp.StatusCode, string(b))
	}
	var tok struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	resp.Body.Close()
	if tok.Access == "" || tok.Refresh == "" {
		t.Skip("missing tokens")
	}

	// Wait for access to expire (+1s)
	time.Sleep(4 * time.Second)
	req, _ := http.NewRequest("GET", baseURL+"/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Access)
	r2, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r2.Body)
	r2.Body.Close()
	if r2.StatusCode != 401 {
		t.Fatalf("expected 401 after access expiry, got %d", r2.StatusCode)
	}

	// Wait for refresh to expire (+1s)
	time.Sleep(2 * time.Second)
	// Try refresh
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", seed.Clients.Web.ClientID)
	form.Set("refresh_token", tok.Refresh)
	r3, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r3.Body)
	r3.Body.Close()
	if r3.StatusCode != 400 && r3.StatusCode != 401 {
		t.Fatalf("expected 400/401 after refresh expiry, got %d", r3.StatusCode)
	}
}
