package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	jwti "github.com/dropDatabas3/hellojohn/test/e2e/internal"
	"github.com/stretchr/testify/require"
)

// 35 - Per-tenant issuer + JWKS verification
func Test_35_PerTenant_Issuer_JWKS(t *testing.T) {
	c := newHTTPClient()

	// Admin access token (seed admin)
	var adminAccess string
	{
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var out struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, mustJSON(resp.Body, &out))
		require.NotEmpty(t, out.AccessToken)
		adminAccess = out.AccessToken
	}

	// 1) Upsert tenant ACME and enable path-based issuer; also ensure user store is configured and migrated
	{
		// Upsert tenant with issuerMode path. Also pass userDB so we can register/login in this tenant.
		// Use same DSN as main env via STORAGE_DSN env propagated by TestMain.
		payload := map[string]any{
			"slug": "acme",
			"name": "Acme Corp",
			"settings": map[string]any{
				"issuerMode": "path",
				// userDB settings use camelCase in Admin FS (provider handles encryption)
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
		resp, err := c.Do(req)
		require.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}

		// Upsert client web-frontend
		cl := map[string]any{
			"clientId":       "web-frontend",
			"name":           "Web Frontend",
			"type":           "public",
			"redirectUris":   []string{baseURL + "/v1/auth/social/result", "http://localhost:7777/callback"},
			"allowedOrigins": []string{"http://localhost:7777"},
			"providers":      []string{"password"},
			"scopes":         []string{"openid", "profile", "email", "offline_access"},
			"enabled":        true,
		}
		cb, _ := json.Marshal(cl)
		rq, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/acme/clients/web-frontend", bytes.NewReader(cb))
		rq.Header.Set("Authorization", "Bearer "+adminAccess)
		rq.Header.Set("Content-Type", "application/json")
		resp2, err := c.Do(rq)
		require.NoError(t, err)
		if resp2 != nil {
			resp2.Body.Close()
		}

		// Test-connection and migrate
		for _, ep := range []string{"test-connection", "migrate"} {
			mr, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants/acme/user-store/"+ep, nil)
			mr.Header.Set("Authorization", "Bearer "+adminAccess)
			res, err := c.Do(mr)
			require.NoError(t, err)
			if res != nil {
				res.Body.Close()
			}
		}
	}

	// Resolve ACME tenant UUID via admin GET
	var acmeID string
	{
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/admin/tenants/acme", nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var out struct {
			ID string `json:"id"`
		}
		require.NoError(t, mustJSON(resp.Body, &out))
		require.NotEmpty(t, out.ID)
		acmeID = out.ID
	}

	// 2) Register user in ACME (idempotent)
	email := uniqueEmail("u@acme.test", "acme")
	{
		body := map[string]any{
			"tenant_id": acmeID,
			"client_id": "web-frontend",
			"email":     email,
			"password":  "Passw0rd!",
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/auth/register", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		require.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}
		// Allow some time for eventual consistency in any caches
		time.Sleep(100 * time.Millisecond)
	}

	// 3) Login in ACME and validate iss ends with /t/acme
	var tok string
	{
		body := map[string]any{
			"tenant_id": acmeID,
			"client_id": "web-frontend",
			"email":     email,
			"password":  "Passw0rd!",
		}
		b, _ := json.Marshal(body)
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(b))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var out struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, mustJSON(resp.Body, &out))
		require.NotEmpty(t, out.AccessToken)
		tok = out.AccessToken
	}
	h, p, err := decodeJWT(tok)
	require.NoError(t, err)
	require.NotEmpty(t, h["kid"])
	iss, _ := p["iss"].(string)
	require.Contains(t, iss, "/t/acme")

	// 4) JWKS per-tenant responds
	resp := mustGet(t, c, baseURL+"/.well-known/jwks/acme.json", http.StatusOK)
	var ks map[string]any
	require.NoError(t, mustJSON(resp.Body, &ks))
	resp.Body.Close()
	require.NotEmpty(t, ks["keys"])

	// 5) Verification with GLOBAL JWKS must fail
	err = jwti.VerifyWithJWKS(baseURL+"/.well-known/jwks.json", tok)
	require.Error(t, err)

	// 6) Verification with TENANT JWKS must succeed
	require.NoError(t, jwti.VerifyWithJWKS(baseURL+"/.well-known/jwks/acme.json", tok))
}

func mustGet(t *testing.T, c *http.Client, url string, want int) *http.Response {
	t.Helper()
	resp, err := c.Get(url)
	require.NoError(t, err)
	require.Equal(t, want, resp.StatusCode)
	return resp
}
