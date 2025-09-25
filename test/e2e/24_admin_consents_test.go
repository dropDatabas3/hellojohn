package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// 24 - Admin Consents: upsert/list/revoke (+ efecto sobre refresh)
func Test_24_Admin_Consents(t *testing.T) {
	c := newHTTPClient()

	// --- login admin ---
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
			t.Fatalf("admin login status=%d", resp.StatusCode)
		}
		var out struct {
			AccessToken string `json:"access_token"`
		}
		_ = mustJSON(resp.Body, &out)
		adminAccess = out.AccessToken
	}

	requireScopes := func(scopes ...string) {
		// best-effort: crear si no existen
		req, _ := http.NewRequest("GET", baseURL+"/v1/admin/scopes?tenant_id="+url.QueryEscape(seed.Tenant.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var arr []map[string]any
		_ = mustJSON(resp.Body, &arr)
		have := map[string]bool{}
		for _, it := range arr {
			have[asString(it["name"])] = true
		}
		for _, s := range scopes {
			if have[s] {
				continue
			}
			body := map[string]any{"tenant_id": seed.Tenant.ID, "name": s, "description": "e2e scope"}
			j, _ := json.Marshal(body)
			rq, _ := http.NewRequest("POST", baseURL+"/v1/admin/scopes", bytes.NewReader(j))
			rq.Header.Set("Authorization", "Bearer "+adminAccess)
			rq.Header.Set("Content-Type", "application/json")
			r, err := c.Do(rq)
			if err == nil {
				io.Copy(io.Discard, r.Body)
				r.Body.Close()
			}
		}
	}

	need := []string{"profile:read", "email:read"}
	requireScopes(need...)

	// --- crear usuario efímero + login para obtener refresh ---
	var userID, refresh string
	{
		email := uniqueEmail(seed.Users.Admin.Email, "cons")
		pass := "S3gura!" + itoa(time.Now().Unix()%1_000_000)

		// register
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
			"email": email, "password": pass,
		})
		resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// puede devolver 200 con tokens o 204 sin cuerpo
		if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("register status=%d body=%s", resp.StatusCode, string(b))
		}

		// login
		body, _ = json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
			"email": email, "password": pass,
		})
		resp2, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != 200 {
			t.Fatalf("user login status=%d", resp2.StatusCode)
		}
		var tok struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := mustJSON(resp2.Body, &tok); err != nil {
			t.Fatal(err)
		}
		refresh = tok.RefreshToken

		// /v1/me para obtener userID (sub)
		req, _ := http.NewRequest("GET", baseURL+"/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		me, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer me.Body.Close()
		if me.StatusCode != 200 {
			t.Fatalf("/v1/me status=%d", me.StatusCode)
		}
		var meOut map[string]any
		if err := mustJSON(me.Body, &meOut); err != nil {
			t.Fatal(err)
		}
		userID = asString(meOut["sub"])
		if userID == "" {
			t.Fatalf("no sub in /v1/me")
		}
	}

	// --- upsert consent (user+client) ---
	t.Run("upsert consent", func(t *testing.T) {
		body := map[string]any{"user_id": userID, "client_id": seed.Clients.Web.ClientID, "scopes": need}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/consents/upsert", bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("upsert consent status=%d body=%s", resp.StatusCode, string(b))
		}
	})

	// --- list by-user active_only ---
	t.Run("list by-user active_only", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/admin/consents/by-user?user_id="+url.QueryEscape(userID)+"&active_only=true", nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("by-user status=%d", resp.StatusCode)
		}
		var arr []map[string]any
		_ = mustJSON(resp.Body, &arr)
		if len(arr) == 0 {
			t.Fatalf("expected at least one consent")
		}
	})

	// --- revoke consent (user+client) ---
	t.Run("revoke consent and refresh should fail", func(t *testing.T) {
		body := map[string]any{"user_id": userID, "client_id": seed.Clients.Web.ClientID}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/consents/revoke", bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			t.Fatalf("revoke consent status=%d", resp.StatusCode)
		}

		// refresh (con token previo) debe fallar 400/401
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		b2, _ := json.Marshal(refreshReq{ClientID: seed.Clients.Web.ClientID, RefreshToken: refresh})
		r2, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(b2))
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()
		if r2.StatusCode != 400 && r2.StatusCode != 401 {
			t.Fatalf("expected 400/401 refresh after consent revoke; got %d", r2.StatusCode)
		}

		// by-user active_only ahora debería estar vacío
		req2, _ := http.NewRequest("GET", baseURL+"/v1/admin/consents/by-user?user_id="+url.QueryEscape(userID)+"&active_only=true", nil)
		req2.Header.Set("Authorization", "Bearer "+adminAccess)
		r3, err := c.Do(req2)
		if err != nil {
			t.Fatal(err)
		}
		defer r3.Body.Close()
		var arr []map[string]any
		_ = mustJSON(r3.Body, &arr)
		if len(arr) != 0 {
			t.Fatalf("expected 0 active consents after revoke; got %d", len(arr))
		}
	})
}
