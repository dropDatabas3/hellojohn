package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_37_Discovery_Per_Tenant(t *testing.T) {
	c := newHTTPClient()

	// 1) Obtener token de admin con el seed
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

	// 2) Forzar issuerMode=path para acme (idempotente)
	{
		payload := map[string]any{
			"slug": "acme",
			"settings": map[string]any{
				"issuerMode": "path",
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
	}

	discoURL := baseURL + "/t/acme/.well-known/openid-configuration"

	// 3) GET discovery por tenant
	req, _ := http.NewRequest("GET", discoURL, nil)
	resp, err := c.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	ct := resp.Header.Get("Content-Type")
	require.Contains(t, ct, "application/json", "content-type debe ser application/json")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))

	require.Equal(t, baseURL+"/t/acme", doc["issuer"])
	require.Equal(t, baseURL+"/.well-known/jwks/acme.json", doc["jwks_uri"])
	require.Equal(t, baseURL+"/oauth2/authorize", doc["authorization_endpoint"]) // rutas globales actuales
	require.Equal(t, baseURL+"/oauth2/token", doc["token_endpoint"])
	require.Equal(t, baseURL+"/userinfo", doc["userinfo_endpoint"])

	// 4) HEAD discovery por tenant
	req, _ = http.NewRequest("HEAD", discoURL, nil)
	resp, err = c.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	// 5) Negativo: tenant inexistente
	badURL := baseURL + "/t/ghost/.well-known/openid-configuration"
	resp, err = c.Get(badURL)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	_ = resp.Body.Close()
}
