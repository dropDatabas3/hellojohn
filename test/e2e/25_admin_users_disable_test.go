package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 25 - Admin Users: disable/enable bloquea/permite login.
// Endpoints esperados (best-effort):
//   - POST /v1/admin/users/disable  {tenant_id, user_id}
//   - POST /v1/admin/users/enable   {tenant_id, user_id}
//
// Si el backend aún no los trae -> skip elegante.
func Test_25_Admin_Users_Disable_Enable(t *testing.T) {
	c := newHTTPClient()

	type tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	// 0) Admin login
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
		adminAccess = out.AccessToken
		if adminAccess == "" {
			t.Fatalf("admin login sin access token")
		}
	}

	// 1) Crear usuario efímero y verificar que puede loguear
	email := strings.ToLower("disable_user_" + time.Now().Format("150405.000000") + "@example.test")
	password := "Adm1n!" + time.Now().Format("1504")

	var userID string

	{
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     email,
			"password":  password,
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

		// Obtener user_id del access token si lo devolvió, caso contrario hacer login y extraerlo
		var tk tokens
		_ = mustJSON(resp.Body, &tk)
		if tk.AccessToken == "" {
			// login
			lb, _ := json.Marshal(map[string]any{
				"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
				"email": email, "password": password,
			})
			r2, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(lb))
			if err != nil {
				t.Fatal(err)
			}
			defer r2.Body.Close()
			if r2.StatusCode != 200 {
				b, _ := io.ReadAll(r2.Body)
				t.Fatalf("login (recién creado) status=%d body=%s", r2.StatusCode, string(b))
			}
			if err := mustJSON(r2.Body, &tk); err != nil {
				t.Fatal(err)
			}
		}
		if tk.AccessToken == "" {
			t.Skip("no obtuvimos access token para extraer sub; skipping")
		}
		// parse sub del access
		_, pld, err := decodeJWT(tk.AccessToken)
		if err != nil {
			t.Fatalf("decode access: %v", err)
		}
		if s, _ := pld["sub"].(string); s != "" {
			userID = s
		}
		if userID == "" {
			t.Skip("no pudimos resolver userID/sub; skipping")
		}
	}

	// helpers disable/enable
	call := func(path string) (int, string) {
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"user_id":   userID,
		})
		req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	// 2) Disable (admin)
	st, body := call("/v1/admin/users/disable")
	if st == 404 || st == 501 {
		t.Skip("disable user no implementado; skipping")
	}
	if st != 200 && st != 204 {
		t.Fatalf("disable status=%d body=%s", st, body)
	}

	// 3) Login debe fallar (401/403)
	{
		lb, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
			"email": email, "password": password,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(lb))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 && resp.StatusCode != 403 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("login (disabled) debería 401/403; got %d body=%s", resp.StatusCode, string(b))
		}
	}

	// 4) Enable (admin)
	st, body = call("/v1/admin/users/enable")
	if st == 404 || st == 501 {
		t.Skip("enable user no implementado; skipping")
	}
	if st != 200 && st != 204 {
		t.Fatalf("enable status=%d body=%s", st, body)
	}

	// 5) Login vuelve a funcionar
	{
		lb, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID,
			"email": email, "password": password,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(lb))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("login (enabled) status=%d body=%s", resp.StatusCode, string(b))
		}
	}
}
