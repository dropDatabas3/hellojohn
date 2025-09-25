package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// 22 - Admin Clients CRUD + revoke + delete (soft/hard) + seguridad
func Test_22_Admin_Clients(t *testing.T) {
	c := newHTTPClient()

	type tokens struct {
		AccessToken string `json:"access_token"`
	}

	// --- login admin (Bearer) ---
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
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("admin login status=%d body=%s", resp.StatusCode, string(b))
		}
		var out tokens
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.AccessToken == "" {
			t.Fatalf("admin login sin access token")
		}
		adminAccess = out.AccessToken
	}

	// --- seguridad: 401 sin token ---
	t.Run("401 without bearer", func(t *testing.T) {
		resp, err := c.Get(baseURL + "/v1/admin/clients?tenant_id=" + url.QueryEscape(seed.Tenant.ID))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401 without bearer; got %d", resp.StatusCode)
		}
	})

	// --- seguridad: 403 con usuario NO-admin (si enforce activo) ---
	t.Run("403 for non-admin (if enforce)", func(t *testing.T) {
		// Registrar usuario efímero
		email := uniqueEmail(seed.Users.Admin.Email, "admcli")
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
			"email": email, "password": "S3guraS3gura!",
		})
		resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var regOut tokens
		_ = mustJSON(resp.Body, &regOut) // algunos backends devuelven 204
		if regOut.AccessToken == "" && resp.StatusCode == 204 {
			// Hacer login para tener access
			body, _ = json.Marshal(map[string]any{
				"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
				"email": email, "password": "S3guraS3gura!",
			})
			resp2, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp2.Body.Close()
			if resp2.StatusCode != 200 {
				t.Skipf("no-admin login failed (%d); skipping 403 test", resp2.StatusCode)
			}
			if err := mustJSON(resp2.Body, &regOut); err != nil || regOut.AccessToken == "" {
				t.Skip("no-admin no obtuvo access; skipping 403 test")
			}
		}
		req, _ := http.NewRequest("GET", baseURL+"/v1/admin/clients?tenant_id="+url.QueryEscape(seed.Tenant.ID), nil)
		req.Header.Set("Authorization", "Bearer "+regOut.AccessToken)
		r, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		// Algunos despliegues pueden estar en modo compat y devolver 2xx; aceptamos 403 o 2xx con log.
		if r.StatusCode == 401 {
			t.Fatalf("got 401 with valid non-admin bearer (revisar issuer/claims)")
		}
		if r.StatusCode == 200 {
			t.Log("admin enforce deshabilitado (modo compat) – listado devolvió 200")
		} else if r.StatusCode != 403 {
			t.Fatalf("esperado 403 (o 200 compat); got %d", r.StatusCode)
		}
	})

	// --- CRUD + revoke ---
	var createdID, createdPublicID string
	ts := time.Now().UnixNano()
	newClientID := "e2e-client-" + itoa(ts)

	t.Run("create client", func(t *testing.T) {
		body := map[string]any{
			"tenant_id":       seed.Tenant.ID,
			"name":            "E2E Client " + itoa(ts),
			"client_id":       newClientID,
			"client_type":     "public",
			"redirect_uris":   []string{},
			"allowed_origins": []string{},
			"providers":       []string{},
			"scopes":          []string{},
		}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/clients", bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 201 && resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("create status=%d body=%s", resp.StatusCode, string(b))
		}
		var out struct {
			ID     string `json:"id"`
			Client struct {
				ID       string `json:"id"`
				ClientID string `json:"client_id"`
			} `json:"client"`
		}
		_ = mustJSON(resp.Body, &out) // algunos backends devuelven shape simple
		createdID = out.ID
		if createdID == "" && out.Client.ID != "" {
			createdID = out.Client.ID
		}
		if createdID == "" {
			t.Fatalf("create sin id en respuesta")
		}
		createdPublicID = newClientID
	})

	t.Run("duplicate client_id -> 409", func(t *testing.T) {
		body := map[string]any{
			"tenant_id":       seed.Tenant.ID,
			"name":            "E2E Client DUP " + itoa(ts),
			"client_id":       newClientID,
			"client_type":     "public",
			"redirect_uris":   []string{},
			"allowed_origins": []string{},
		}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/clients", bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 409 && resp.StatusCode != 400 {
			t.Fatalf("expected 409 or 400 on duplicate client_id; got %d", resp.StatusCode)
		}
	})

	t.Run("list with q filter includes created", func(t *testing.T) {
		q := url.QueryEscape("E2E Client")
		req, _ := http.NewRequest("GET", baseURL+"/v1/admin/clients?tenant_id="+url.QueryEscape(seed.Tenant.ID)+"&q="+q, nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("list status=%d", resp.StatusCode)
		}
		var arr []map[string]any
		_ = mustJSON(resp.Body, &arr)
		found := false
		for _, it := range arr {
			if asString(it["id"]) == createdID || asString(it["client_id"]) == createdPublicID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("created client not found in list")
		}
	})

	t.Run("get by id", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/admin/clients/"+createdID, nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("get by id status=%d body=%s", resp.StatusCode, string(b))
		}
		var got struct {
			Client struct {
				ID           string   `json:"id"`
				Name         string   `json:"name"`
				ClientType   string   `json:"client_type"`
				RedirectURIs []string `json:"redirect_uris"`
			} `json:"client"`
		}
		_ = mustJSON(resp.Body, &got)
		if got.Client.ID != createdID {
			t.Fatalf("id mismatch, got %s want %s", got.Client.ID, createdID)
		}
	})

	t.Run("update client fields", func(t *testing.T) {
		body := map[string]any{
			"name":            "E2E Client UPDATED " + itoa(ts),
			"client_type":     "confidential",
			"redirect_uris":   []string{"https://example.local/cb"},
			"allowed_origins": []string{"https://example.local"},
			"providers":       []string{"password"},
			"scopes":          []string{"openid", "profile:read"},
		}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", baseURL+"/v1/admin/clients/"+createdID, bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 204 {
			t.Fatalf("update status=%d", resp.StatusCode)
		}

		// get post-update
		req2, _ := http.NewRequest("GET", baseURL+"/v1/admin/clients/"+createdID, nil)
		req2.Header.Set("Authorization", "Bearer "+adminAccess)
		r2, err := c.Do(req2)
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()
		if r2.StatusCode != 200 {
			t.Fatalf("get post-update status=%d", r2.StatusCode)
		}
		var got struct {
			Client struct {
				ClientType   string   `json:"client_type"`
				Name         string   `json:"name"`
				RedirectURIs []string `json:"redirect_uris"`
			} `json:"client"`
		}
		_ = mustJSON(r2.Body, &got)
		if got.Client.ClientType != "confidential" {
			t.Fatalf("client_type not updated (got %s)", got.Client.ClientType)
		}
		if !contains(got.Client.RedirectURIs, "https://example.local/cb") {
			t.Fatalf("redirect_uris not updated")
		}
	})

	t.Run("revoke all refresh by client", func(t *testing.T) {
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/clients/"+createdID+"/revoke", nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			t.Fatalf("revoke status=%d", resp.StatusCode)
		}
	})

	t.Run("soft delete (keeps record)", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/v1/admin/clients/"+createdID+"?soft=true", nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			t.Fatalf("soft delete status=%d", resp.StatusCode)
		}

		req2, _ := http.NewRequest("GET", baseURL+"/v1/admin/clients/"+createdID, nil)
		req2.Header.Set("Authorization", "Bearer "+adminAccess)
		r2, err := c.Do(req2)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r2.Body)
		r2.Body.Close()
		if r2.StatusCode != 200 {
			t.Fatalf("get after soft delete should exist; got %d", r2.StatusCode)
		}
	})

	t.Run("hard delete (removes record)", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/v1/admin/clients/"+createdID, nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			t.Fatalf("hard delete status=%d", resp.StatusCode)
		}

		req2, _ := http.NewRequest("GET", baseURL+"/v1/admin/clients/"+createdID, nil)
		req2.Header.Set("Authorization", "Bearer "+adminAccess)
		r2, err := c.Do(req2)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r2.Body)
		r2.Body.Close()
		if r2.StatusCode != 404 {
			t.Fatalf("get after hard delete should be 404; got %d", r2.StatusCode)
		}
	})
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
