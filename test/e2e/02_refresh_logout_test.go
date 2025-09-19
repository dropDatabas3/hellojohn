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

// 02 - Refresh & Logout (rotación, replay protection, revocación)
func Test_02_Refresh_And_Logout(t *testing.T) {
	c := newHTTPClient()

	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}

	// --- login con usuario admin del seed ---
	var access, refresh string
	{
		body, _ := json.Marshal(loginReq{
			TenantID: seed.Tenant.ID,
			ClientID: seed.Clients.Web.ClientID,
			Email:    seed.Users.Admin.Email,
			Password: seed.Users.Admin.Password,
		})
		resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("login status=%d body=%s", resp.StatusCode, string(b))
		}
		var out tok
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.AccessToken == "" || out.RefreshToken == "" {
			t.Fatalf("login missing tokens: %+v", out)
		}
		access, refresh = out.AccessToken, out.RefreshToken
	}

	// --- /v1/me OK con access ---
	t.Run("me with access", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("/v1/me status=%d", resp.StatusCode)
		}
	})

	// --- refresh: rota y retorna headers no-store/no-cache ---
	var newAccess, newRefresh string
	t.Run("refresh rotates and sets no-store/no-cache", func(t *testing.T) {
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(refreshReq{
			ClientID:     seed.Clients.Web.ClientID,
			RefreshToken: refresh,
		})
		resp, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("refresh status=%d body=%s", resp.StatusCode, string(b))
		}
		// seguridad: no almacenar
		if cc := readHeader(resp, "Cache-Control"); !strings.EqualFold(cc, "no-store") {
			t.Fatalf("Cache-Control expected no-store, got %q", cc)
		}
		if pg := readHeader(resp, "Pragma"); !strings.EqualFold(pg, "no-cache") {
			t.Fatalf("Pragma expected no-cache, got %q", pg)
		}
		var out tok
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.RefreshToken == "" || out.AccessToken == "" {
			t.Fatalf("refresh missing tokens: %+v", out)
		}
		if out.RefreshToken == refresh {
			t.Fatalf("refresh token did not rotate")
		}
		newAccess, newRefresh = out.AccessToken, out.RefreshToken
	})

	// --- reuse del refresh anterior: debe fallar (401) ---
	t.Run("reuse old refresh -> 401", func(t *testing.T) {
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(refreshReq{
			ClientID:     seed.Clients.Web.ClientID,
			RefreshToken: refresh, // old
		})
		resp, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 401; got %d body=%s", resp.StatusCode, string(b))
		}
	})

	// --- logout revoca refresh actual ---
	t.Run("logout revokes refresh", func(t *testing.T) {
		type logoutReq struct {
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(logoutReq{RefreshToken: newRefresh})
		resp, err := c.Post(baseURL+"/v1/auth/logout", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			t.Fatalf("logout status=%d", resp.StatusCode)
		}
		// refresh tras logout -> 401
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		b2, _ := json.Marshal(refreshReq{ClientID: seed.Clients.Web.ClientID, RefreshToken: newRefresh})
		r2, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(b2))
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()
		if r2.StatusCode != 401 {
			t.Fatalf("expected 401 after logout, got %d", r2.StatusCode)
		}
	})

	// --- negativos de entrada (client_id faltante / inválido) ---
	t.Run("refresh missing client_id -> 400/401", func(t *testing.T) {
		type refreshReq struct {
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(refreshReq{RefreshToken: newRefresh})
		resp, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// aceptamos 400 o 401 según implementación
		if resp.StatusCode != 400 && resp.StatusCode != 401 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400/401 missing client_id; got %d body=%s", resp.StatusCode, string(b))
		}
	})

	t.Run("refresh invalid client_id -> 400/401", func(t *testing.T) {
		type refreshReq struct {
			ClientID     string `json:"client_id"`
			RefreshToken string `json:"refresh_token"`
		}
		body, _ := json.Marshal(refreshReq{
			ClientID:     "no-such-client",
			RefreshToken: newRefresh,
		})
		resp, err := c.Post(baseURL+"/v1/auth/refresh", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 && resp.StatusCode != 401 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400/401 invalid client; got %d body=%s", resp.StatusCode, string(b))
		}
	})

	// sanity: /v1/me con el access post-rotación (antes del logout) funcionaba;
	// tras logout, ese access puede seguir válido hasta expirar (depende de diseño).
	_ = newAccess
	_ = time.Now // evitar warnings si no se usa en algunas builds
}
