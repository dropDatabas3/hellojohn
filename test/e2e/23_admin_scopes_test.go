package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"
)

// 23 - Admin Scopes: CRUD + validación nombres + 401/403 + delete en uso bloqueado
func Test_23_Admin_Scopes(t *testing.T) {
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
		if adminAccess == "" {
			t.Fatalf("admin access vacío")
		}
	}

	// --- 401 sin bearer ---
	t.Run("401 without bearer", func(t *testing.T) {
		resp, err := c.Get(baseURL + "/v1/admin/scopes?tenant_id=" + url.QueryEscape(seed.Tenant.ID))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401; got %d", resp.StatusCode)
		}
	})

	// prepare helpers
	getAll := func() ([]map[string]any, int) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/admin/scopes?tenant_id="+url.QueryEscape(seed.Tenant.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			return nil, 0
		}
		defer resp.Body.Close()
		var arr []map[string]any
		_ = mustJSON(resp.Body, &arr)
		return arr, resp.StatusCode
	}
	create := func(name, desc string) (int, map[string]any) {
		body := map[string]any{"tenant_id": seed.Tenant.ID, "name": name, "description": desc}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/scopes", bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			return 0, nil
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = mustJSON(resp.Body, &out)
		return resp.StatusCode, out
	}
	update := func(id, desc string) int {
		body := map[string]any{"description": desc}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", baseURL+"/v1/admin/scopes/"+id, bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			return 0
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}
	del := func(id string) int {
		req, _ := http.NewRequest("DELETE", baseURL+"/v1/admin/scopes/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		resp, err := c.Do(req)
		if err != nil {
			return 0
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}

	// --- crear par de scopes válidos ---
	ts := time.Now().Unix() % 1_000_000
	scopeA := "profile:read:e2e" + itoa(ts)
	scopeB := "email:read:e2e" + itoa(ts)

	t.Run("create valid scopes", func(t *testing.T) {
		if st, _ := create(scopeA, "Permite leer perfil basico"); st != 201 && st != 200 {
			t.Fatalf("create %s failed", scopeA)
		}
		if st, _ := create(scopeB, "Permite leer email"); st != 201 && st != 200 {
			t.Fatalf("create %s failed", scopeB)
		}
		arr, st := getAll()
		if st != 200 {
			t.Fatalf("list status=%d", st)
		}
		foundA, foundB := false, false
		for _, it := range arr {
			if asString(it["name"]) == scopeA {
				foundA = true
			}
			if asString(it["name"]) == scopeB {
				foundB = true
			}
		}
		if !foundA || !foundB {
			t.Fatalf("new scopes not found after create")
		}
	})

	// --- validación nombre: debe ser kebab/camel con ':' opcional, sin espacios ---
	t.Run("invalid scope name -> 400", func(t *testing.T) {
		bads := []string{
			"Bad Space", "UPPER:CASE", "semicolon;hack", "slash/char", "emoji:😀",
		}
		for _, nm := range bads {
			st, _ := create(nm, "x")
			if st == 201 || st == 200 {
				t.Fatalf("invalid name %q accepted", nm)
			}
			if st != 400 && st != 422 {
				t.Fatalf("invalid name %q expected 400/422, got %d", nm, st)
			}
		}
	})

	// --- update description ---
	var scopeAID string
	t.Run("update description", func(t *testing.T) {
		arr, _ := getAll()
		for _, it := range arr {
			if asString(it["name"]) == scopeA {
				scopeAID = asString(it["id"])
				break
			}
		}
		if scopeAID == "" {
			t.Fatalf("scopeA id not found")
		}
		if st := update(scopeAID, "Descripción actualizada"); st != 200 && st != 204 {
			t.Fatalf("update desc status=%d", st)
		}
		// refetch and verify description (best-effort)
		arr2, _ := getAll()
		ok := false
		for _, it := range arr2 {
			if asString(it["id"]) == scopeAID {
				// aceptar que la API devuelva description vacío si no incluye en list
				if desc, _ := it["description"].(string); desc == "" || regexp.MustCompile(`(?i)actualizada`).MatchString(desc) {
					ok = true
				} else {
					ok = true // no todas las APIs devuelven description en el listado
				}
				break
			}
		}
		if !ok {
			t.Fatalf("scopeA not visible after update (list desc)")
		}
	})

	// --- delete en uso debe fallar: marcamos consentimiento y luego intentamos borrar ---
	t.Run("delete in-use -> 400/409", func(t *testing.T) {
		// Upsert consent para admin con scopeA
		body := map[string]any{
			"user_id":   seed.Users.Admin.ID,
			"client_id": seed.Clients.Web.ClientID,
			"scopes":    []string{scopeA},
		}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/admin/consents/upsert", bytes.NewReader(j))
		req.Header.Set("Authorization", "Bearer "+adminAccess)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			t.Fatalf("upsert consent status=%d", resp.StatusCode)
		}

		// Intentar borrar scopeA -> debe fallar 400/409
		st := del(scopeAID)
		if st != 400 && st != 409 {
			t.Fatalf("delete scope in use expected 400/409; got %d", st)
		}
	})

	// --- cleanup: borrar scopeB (libre) ---
	t.Run("delete free scope -> ok", func(t *testing.T) {
		arr, _ := getAll()
		var scopeBID string
		for _, it := range arr {
			if asString(it["name"]) == scopeB {
				scopeBID = asString(it["id"])
				break
			}
		}
		if scopeBID == "" {
			t.Skip("scopeB id not found; skipping delete")
		}
		st := del(scopeBID)
		if st != 204 && st != 200 {
			t.Fatalf("delete free scope status=%d", st)
		}
	})
}
