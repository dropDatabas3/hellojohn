package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// 19 - Cross-tenant negative: a refresh from tenant A must not be usable with client_id from tenant B
func Test_19_CrossTenant_Refresh_Negative(t *testing.T) {
	c := newHTTPClient()

	// 1) Create a second tenant via admin FS API
	//    Requires sys:admin auth; we'll login as admin first
	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	lb, _ := json.Marshal(loginReq{
		TenantID: seed.Tenant.ID,
		ClientID: seed.Clients.Web.ClientID,
		Email:    seed.Users.Admin.Email,
		Password: seed.Users.Admin.Password,
	})
	lr, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(lb))
	if err != nil {
		t.Fatal(err)
	}
	defer lr.Body.Close()
	if lr.StatusCode != 200 {
		b, _ := io.ReadAll(lr.Body)
		t.Fatalf("admin login status=%d body=%s", lr.StatusCode, string(b))
	}
	var ltok tokens
	if err := mustJSON(lr.Body, &ltok); err != nil || ltok.AccessToken == "" {
		t.Fatalf("admin login missing access: %v", err)
	}

	// 2) Create tenant B (slug "other") if not exists
	tenantSlug := "other"
	{
		body := map[string]string{"name": "Other Tenant", "slug": tenantSlug}
		bb, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants", bytes.NewReader(bb))
		req.Header.Set("Authorization", "Bearer "+ltok.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// If already exists, GET should work later; accept 200/201/409
		if resp.StatusCode != 201 && resp.StatusCode != 200 && resp.StatusCode != 409 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("create tenant other status=%d body=%s", resp.StatusCode, string(b))
		}
	}

	// 3) Create a client under tenant B via admin FS clients
	clientB := "web-other"
	{
		in := map[string]any{
			"client_id":       clientB,
			"name":            "Web Other",
			"client_type":     "public",
			"redirect_uris":   []string{baseURL + "/callback"},
			"allowed_origins": []string{"http://localhost:3000"},
			"providers":       []string{"password"},
			"scopes":          []string{"openid", "email", "profile", "offline_access"},
		}
		bb, _ := json.Marshal(in)
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/clients?tenant="+tenantSlug, bytes.NewReader(bb))
		req.Header.Set("Authorization", "Bearer "+ltok.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			b, _ := io.ReadAll(resp.Body)
			// Allow conflict-like semantics if client already exists
			if resp.StatusCode != 400 {
				t.Fatalf("create client other status=%d body=%s", resp.StatusCode, string(b))
			}
		}
	}

	// 4) Login in tenant A to get a refresh
	var refreshA string
	{
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(lb))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("login A status=%d body=%s", resp.StatusCode, string(b))
		}
		var out tokens
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.RefreshToken == "" {
			t.Fatalf("missing refresh from tenant A login")
		}
		refreshA = out.RefreshToken
	}

	// 5) Try to refresh using client_id from tenant B -> must be 401
	{
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(refreshReq{ClientID: clientB, RefreshToken: refreshA})
		resp, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 401 cross-tenant misuse; got %d body=%s", resp.StatusCode, string(b))
		}
	}
}
