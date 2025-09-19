package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 16 - Logout All multi-sesión: usuario temporal, 2 refresh tokens, logout_all revoca ambos.
func Test_16_LogoutAll(t *testing.T) {
	if seed == nil {
		t.Skip("seed vacío")
	}
	c := newHTTPClient()

	email := strings.ToLower("logout_all_" + time.Now().Format("150405.000000") + "@example.test")
	password := "Adm1n!" + time.Now().Format("1504") // evitar blacklist simple

	// 1) Registrar usuario (auto-login produce access/refresh)
	reg := map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     email,
		"password":  password,
	}
	b, _ := json.Marshal(reg)
	resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("register status=%d body=%s", resp.StatusCode, string(body))
	}
	var regOut struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := mustJSON(resp.Body, &regOut); err != nil {
		t.Fatal(err)
	}
	if regOut.AccessToken == "" || regOut.RefreshToken == "" {
		t.Fatalf("registro sin tokens")
	}

	// Extraer user_id (sub) del JWT access
	parts := strings.Split(regOut.AccessToken, ".")
	if len(parts) < 2 {
		t.Fatalf("jwt malformado")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		t.Fatalf("claims json: %v", err)
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		t.Fatalf("sin sub en access token")
	}

	// Helper login (crea nuevas sesiones)
	login := func() (access, refresh string) {
		lr := map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     email,
			"password":  password,
		}
		b2, _ := json.Marshal(lr)
		resp2, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(b2))
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != 200 {
			body, _ := io.ReadAll(resp2.Body)
			t.Fatalf("login status=%d body=%s", resp2.StatusCode, string(body))
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := mustJSON(resp2.Body, &out); err != nil {
			t.Fatal(err)
		}
		return out.AccessToken, out.RefreshToken
	}

	_, rt2 := login() // segunda sesión
	_, rt3 := login() // tercera sesión
	if rt2 == regOut.RefreshToken || rt3 == regOut.RefreshToken || rt2 == rt3 {
		t.Fatalf("refresh tokens deberían ser únicos")
	}

	// 2) logout_all (revoca TODAS las sesiones del usuario)
	reqBody, _ := json.Marshal(map[string]any{"user_id": sub})
	req, _ := http.NewRequest("POST", baseURL+"/v1/auth/logout-all", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	respLA, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer respLA.Body.Close()
	if respLA.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(respLA.Body)
		t.Fatalf("logout_all status=%d body=%s", respLA.StatusCode, string(body))
	}

	// 3) Todos los refresh deben quedar inválidos
	tryRefresh := func(rt string) int {
		form := "grant_type=refresh_token&client_id=" + seed.Clients.Web.ClientID + "&refresh_token=" + rt
		r, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", bytes.NewBufferString(form))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		return r.StatusCode
	}
	for i, rt := range []string{regOut.RefreshToken, rt2, rt3} {
		if st := tryRefresh(rt); st == 200 {
			t.Fatalf("refresh #%d aún válido tras logout_all", i+1)
		}
	}
}
