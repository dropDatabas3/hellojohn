package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
)

// This test validates that in FS-only mode (no per-tenant DB configured),
// auth endpoints gate with 501 Not Implemented using tenant_db_missing. After
// configuring DSN and calling migrate, flows succeed.
func Test34_FSOnly_Gating(t *testing.T) {
	c := newHTTPClient()

	// Ensure tenant 'acme' exists in FS with no DB configured yet so runtime returns 501 (tenant_db_missing)
	// Acquire admin access token via seed admin
	var adminAccess string
	{
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("admin login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("admin login status=%d", resp.StatusCode)
		}
		var out struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.AccessToken == "" {
			t.Fatalf("admin login missing access_token")
		}
		adminAccess = out.AccessToken
	}
	{
		// Upsert tenant 'acme' with empty settings (no userDB)
		payload := map[string]any{"id": "acme", "slug": "acme", "name": "Acme Corp", "settings": map[string]any{}}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/acme", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
			if resp != nil {
				resp.Body.Close()
			}
			t.Fatalf("upsert tenant acme failed")
		}
		resp.Body.Close()
	}

	// 1) Hit login/register with a tenant that has no DB yet => expect 501
	body := map[string]any{
		"tenant_id": "acme",
		"client_id": "web",
		"email":     "a@acme.io",
		"password":  "Password!123",
	}
	buf, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/auth/register", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501 before tenant DB configured, got %d", resp.StatusCode)
	}

	// 2) Configure tenant DB via admin settings then call migrate
	// NOTE: The test harness seeds a tenant 'acme' with encrypted DSN if TEST_PG_DSN is set.
	// If not available in this environment, we skip the rest.
	testPGDSN := os.Getenv("TEST_PG_DSN")
	if testPGDSN == "" {
		t.Skip("TEST_PG_DSN not configured; skipping migration enablement")
		return
	}

	// Update settings: this assumes the admin_tenants_fs handler is present and test harness
	// has sysadmin token baked via TestMain helpers.
	// acquire admin access token via login
	{
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("admin login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("admin login status=%d body=%s", resp.StatusCode, string(b))
		}
		var out struct {
			AccessToken string `json:"access_token"`
		}
		if err := mustJSON(resp.Body, &out); err != nil || out.AccessToken == "" {
			t.Fatalf("admin login without access token")
		}
		adminAccess = out.AccessToken
	}

	// PUT /v1/admin/tenants/acme/settings with If-Match
	// First GET to retrieve ETag
	g1, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/admin/tenants/acme/settings", nil)
	g1.Header.Set("Authorization", "Bearer "+adminAccess)
	r1, err := c.Do(g1)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("get settings status=%d", r1.StatusCode)
	}
	etag := r1.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("missing ETag")
	}

	// Compose new settings with encrypted DSN
	newSettings := map[string]any{
		"user_db": map[string]any{
			"driver": "pg",
			// update using plain DSN; provider will encrypt automatically
			"dsn":    testPGDSN,
			"schema": "public",
		},
	}
	nb, _ := json.Marshal(newSettings)
	p1, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/acme/settings", bytes.NewReader(nb))
	p1.Header.Set("Authorization", "Bearer "+adminAccess)
	p1.Header.Set("If-Match", etag)
	p1.Header.Set("Content-Type", "application/json")
	r2, err := c.Do(p1)
	if err != nil {
		t.Fatalf("put settings: %v", err)
	}
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("put settings status=%d", r2.StatusCode)
	}

	// POST /v1/admin/tenants/acme/user-store/migrate
	mreq, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants/acme/user-store/migrate", nil)
	mreq.Header.Set("Authorization", "Bearer "+adminAccess)
	mr, err := c.Do(mreq)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if mr.StatusCode != http.StatusNoContent {
		t.Fatalf("migrate status=%d", mr.StatusCode)
	}

	// 3) Retry register; should now succeed
	req2, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/auth/register", bytes.NewReader(buf))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := c.Do(req2)
	if err != nil {
		t.Fatalf("register after migrate: %v", err)
	}
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected success after migrate, got %d", resp2.StatusCode)
	}
}
