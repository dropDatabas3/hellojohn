package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// 31 - Refresh rotation race: two concurrent refresh with same RT
func Test_31_Refresh_Race(t *testing.T) {
	c := newHTTPClient()
	// First obtain refresh token via password login
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
		t.Skipf("auth/login not available or failed: %d %s", resp.StatusCode, string(b))
	}
	var out struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if out.RefreshToken == "" {
		t.Skip("no refresh token from login; skip")
	}

	// Prepare the refresh form
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", seed.Clients.Web.ClientID)
	form.Set("refresh_token", out.RefreshToken)

	// Fire two concurrent requests
	type res struct {
		status int
		body   string
	}
	var r1, r2 res
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		rr, e := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if e != nil {
			r1 = res{0, e.Error()}
			return
		}
		b, _ := io.ReadAll(rr.Body)
		rr.Body.Close()
		r1 = res{rr.StatusCode, string(b)}
	}()
	go func() {
		defer wg.Done()
		rr, e := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if e != nil {
			r2 = res{0, e.Error()}
			return
		}
		b, _ := io.ReadAll(rr.Body)
		rr.Body.Close()
		r2 = res{rr.StatusCode, string(b)}
	}()
	wg.Wait()

	// Exactly one should be 200, the other 400/401
	success := 0
	if r1.status == 200 {
		success++
	}
	if r2.status == 200 {
		success++
	}
	if success != 1 {
		t.Fatalf("expected exactly one success; got r1=%d r2=%d", r1.status, r2.status)
	}
}
