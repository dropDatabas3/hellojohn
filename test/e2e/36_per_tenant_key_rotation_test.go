package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

// 36 - Per-tenant key rotation (skeleton). When rotate is implemented, assert KID changes and both verify during the window.
func Test_36_PerTenant_Key_Rotation(t *testing.T) {
	c := newHTTPClient()

	// Ensure ACME exists with path issuer and userDB configured
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
		if err := mustJSON(resp.Body, &out); err != nil || out.AccessToken == "" {
			t.Fatalf("admin login missing access_token")
		}
		adminAccess = out.AccessToken
	}

	{
		payload := map[string]any{
			"slug": "acme",
			"name": "Acme Corp",
			"settings": map[string]any{
				"issuerMode": "path",
				"userDB": map[string]any{
					"type": "postgres",
					"dsn":  os.Getenv("STORAGE_DSN"),
				},
			},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/acme", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		if resp, err := c.Do(req); err == nil && resp != nil {
			resp.Body.Close()
		}
		for _, ep := range []string{"test-connection", "migrate"} {
			mr, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants/acme/user-store/"+ep, nil)
			mr.Header.Set("Authorization", "Bearer "+adminAccess)
			if res, err := c.Do(mr); err == nil && res != nil {
				res.Body.Close()
			}
		}

		cl := map[string]any{
			"clientId":       "web-frontend",
			"name":           "Web Frontend",
			"type":           "public",
			"redirectUris":   []string{baseURL + "/v1/auth/social/result"},
			"allowedOrigins": []string{"http://localhost:7777"},
			"providers":      []string{"password"},
			"scopes":         []string{"openid", "profile", "email", "offline_access"},
			"enabled":        true,
		}
		cb, _ := json.Marshal(cl)
		rq, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/acme/clients/web-frontend", bytes.NewReader(cb))
		rq.Header.Set("Authorization", "Bearer "+adminAccess)
		rq.Header.Set("Content-Type", "application/json")
		if resp2, err := c.Do(rq); err == nil && resp2 != nil {
			resp2.Body.Close()
		}
	}

	// Resolve ACME tenant UUID for auth requests
	var acmeID string
	{
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/admin/tenants/acme", nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err == nil && resp != nil {
			defer resp.Body.Close()
			var out struct {
				ID string `json:"id"`
			}
			_ = mustJSON(resp.Body, &out)
			acmeID = out.ID
		}
	}

	// Login twice with delay and compare KIDs; leave assertion pending until real rotate endpoint exists
	email := uniqueEmail("u@acme.test", "acme-rot")
	reg := map[string]any{"tenant_id": acmeID, "client_id": "web-frontend", "email": email, "password": "Passw0rd!"}
	if b, _ := json.Marshal(reg); true {
		rq, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/auth/register", bytes.NewReader(b))
		rq.Header.Set("Content-Type", "application/json")
		if r, err := c.Do(rq); err == nil && r != nil {
			r.Body.Close()
		}
	}

	tok1 := loginPassword(t, acmeID, "web-frontend", email, "Passw0rd!")
	kid1 := getKID(tok1)

	// TODO: when rotate endpoint exists, call it here
	// POST /v1/admin/tenants/acme/keys/rotate

	time.Sleep(1 * time.Second)
	tok2 := loginPassword(t, acmeID, "web-frontend", email, "Passw0rd!")
	kid2 := getKID(tok2)

	_ = kid1
	_ = kid2
	// When rotate is real: require.NotEqual(t, kid1, kid2)
}

// local helpers to avoid repeating patterns in e2e
func loginPassword(t *testing.T, tenant, client, email, pass string) string {
	t.Helper()
	c := newHTTPClient()
	b, _ := json.Marshal(map[string]any{"tenant_id": tenant, "client_id": client, "email": email, "password": pass})
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := mustJSON(resp.Body, &out); err != nil {
		t.Fatalf("login json: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatalf("missing access token")
	}
	return out.AccessToken
}

func getKID(tok string) string {
	h, _ := jwtHeaderPayload(tok)
	if h == nil {
		return ""
	}
	if s, _ := h["kid"].(string); s != "" {
		return s
	}
	return ""
}
