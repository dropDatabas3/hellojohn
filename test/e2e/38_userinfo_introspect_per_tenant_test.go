package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	jwti "github.com/dropDatabas3/hellojohn/test/e2e/internal"
	"github.com/stretchr/testify/require"
)

// 38 - userinfo/introspect validate per tenant (happy and negatives) + rotation grace acceptance
func Test_38_Userinfo_Introspect_PerTenant(t *testing.T) {
	c := newHTTPClient()

	// Admin login (seed admin)
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

	// Upsert two tenants: acme (path issuer) and beta (path issuer)
	for _, slug := range []string{"acme2", "beta2"} {
		payload := map[string]any{
			"slug": slug,
			"name": slug + " Inc",
			"settings": map[string]any{
				"issuerMode": "path",
				"userDB": map[string]any{ // needed to register/login
					"type": "postgres",
					"dsn":  os.Getenv("STORAGE_DSN"),
				},
			},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/"+slug, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		require.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}

		// client
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
		rq, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/admin/tenants/"+slug+"/clients/web-frontend", bytes.NewReader(cb))
		rq.Header.Set("Authorization", "Bearer "+adminAccess)
		rq.Header.Set("Content-Type", "application/json")
		resp2, err := c.Do(rq)
		require.NoError(t, err)
		if resp2 != nil {
			resp2.Body.Close()
		}

		// test-connection + migrate
		for _, ep := range []string{"test-connection", "migrate"} {
			mr, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants/"+slug+"/user-store/"+ep, nil)
			mr.Header.Set("Authorization", "Bearer "+adminAccess)
			res, err := c.Do(mr)
			require.NoError(t, err)
			if res != nil {
				res.Body.Close()
			}
		}
	}

	// Resolve IDs
	getID := func(slug string) string {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/admin/tenants/"+slug, nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var out struct {
			ID string `json:"id"`
		}
		require.NoError(t, mustJSON(resp.Body, &out))
		require.NotEmpty(t, out.ID)
		return out.ID
	}
	acmeID := getID("acme2")
	betaID := getID("beta2")

	// Register and login in each tenant
	login := func(tenantID, email string) string {
		b, _ := json.Marshal(map[string]any{"tenant_id": tenantID, "client_id": "web-frontend", "email": email, "password": "Passw0rd!"})
		rq, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/auth/register", bytes.NewReader(b))
		rq.Header.Set("Content-Type", "application/json")
		if r, err := c.Do(rq); err == nil && r != nil {
			r.Body.Close()
		}
		// login
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(b))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var out struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, mustJSON(resp.Body, &out))
		require.NotEmpty(t, out.AccessToken)
		return out.AccessToken
	}

	tokA := login(acmeID, uniqueEmail("u@acme.test", "acme2"))
	_ = login(betaID, uniqueEmail("u@beta.test", "beta2"))

	// Happy path: userinfo/introspect accept tokens for their own tenant
	{
		// userinfo A
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+tokA)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
		// introspect (basic user/pass from env)
		form := urlValues(map[string]string{"token": tokA})
		ireq, _ := http.NewRequest(http.MethodPost, baseURL+"/oauth2/introspect", bytes.NewBufferString(form))
		ireq.Header.Set("Authorization", "Basic "+basicAuth(os.Getenv("INTROSPECT_BASIC_USER"), os.Getenv("INTROSPECT_BASIC_PASS")))
		ireq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		iresp, err := c.Do(ireq)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, iresp.StatusCode)
		iresp.Body.Close()
	}

	// Negative: token with tid of A but iss of B should fail userinfo (tampered token => signature mismatch)
	{
		// Take tokA and replace payload's iss with beta2; keep header/signature to force signature mismatch
		hdr, pld, err := decodeJWT(tokA)
		require.NoError(t, err)
		pld["iss"] = strings.Replace(asString(pld["iss"]), "/t/acme2", "/t/beta2", 1)
		hb, _ := json.Marshal(hdr)
		pb, _ := json.Marshal(pld)
		tampered := b64url(hb) + "." + b64url(pb) + "." + strings.Split(tokA, ".")[2]
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+tampered)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	}

	// Negative: token from A verified against B's JWKS must fail (simulates endpoints that force B)
	{
		require.Error(t, jwti.VerifyWithJWKS(baseURL+"/.well-known/jwks/beta2.json", tokA))
	}

	// Rotation grace: rotate keys for acme2 and ensure userinfo/introspect accept both KIDs
	{
		// Record current kid
		kid1 := GetKID(t, tokA)
		// rotate
		rq, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/admin/tenants/acme2/keys/rotate", bytes.NewReader([]byte(`{"graceSeconds":5}`)))
		rq.Header.Set("Authorization", "Bearer "+adminAccess)
		rq.Header.Set("Content-Type", "application/json")
		r, err := c.Do(rq)
		require.NoError(t, err)
		if r != nil {
			r.Body.Close()
		}

		// wait a bit for rotation to persist
		time.Sleep(200 * time.Millisecond)

		// login again to get new kid
		tokA2 := login(acmeID, uniqueEmail("u@acme.test", "acme2b"))
		kid2 := GetKID(t, tokA2)
		require.NotEqual(t, kid1, kid2)

		// userinfo must accept both during grace
		for _, tk := range []string{tokA, tokA2} {
			req, _ := http.NewRequest(http.MethodGet, baseURL+"/userinfo", nil)
			req.Header.Set("Authorization", "Bearer "+tk)
			resp, err := c.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()
		}
		// introspect must accept both
		for _, tk := range []string{tokA, tokA2} {
			form := urlValues(map[string]string{"token": tk})
			ireq, _ := http.NewRequest(http.MethodPost, baseURL+"/oauth2/introspect", bytes.NewBufferString(form))
			ireq.Header.Set("Authorization", "Basic "+basicAuth(os.Getenv("INTROSPECT_BASIC_USER"), os.Getenv("INTROSPECT_BASIC_PASS")))
			ireq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			iresp, err := c.Do(ireq)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, iresp.StatusCode)
			var out map[string]any
			require.NoError(t, mustJSON(iresp.Body, &out))
			iresp.Body.Close()
			require.Equal(t, true, out["active"]) // accepted
		}
	}
}

// helpers
func basicAuth(user, pass string) string {
	creds := user + ":" + pass
	return base64.StdEncoding.EncodeToString([]byte(creds))
}
func urlValues(m map[string]string) string {
	// super small x-www-form-urlencoded builder
	var buf bytes.Buffer
	first := true
	for k, v := range m {
		if !first {
			buf.WriteByte('&')
		} else {
			first = false
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
	}
	return buf.String()
}
