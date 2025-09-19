package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// 15 - Trusted Device Skip: después de un login con MFA + remember_device, un segundo login debe omitir mfa_required
func Test_15_TrustedDeviceSkip(t *testing.T) {
	if seed == nil || seed.Users.MFA.Email == "" {
		t.Skip("seed.Users.MFA vacío; skipping")
	}
	secret := totpSecretFromSeed(t)
	c := newHTTPClient()

	// 1) Primer login (debe gatillar MFA)
	lr := map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.MFA.Email,
		"password":  seed.Users.MFA.Password,
	}
	body, _ := json.Marshal(lr)
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login1 status=%d body=%s", resp.StatusCode, string(b))
	}
	var lout struct {
		MFAToken string `json:"mfa_token"`
		Required bool   `json:"mfa_required"`
	}
	if err := mustJSON(resp.Body, &lout); err != nil {
		t.Fatal(err)
	}
	if !lout.Required || lout.MFAToken == "" {
		t.Fatalf("se esperaba mfa_required en primer login")
	}

	// 2) Challenge con remember_device=true
	code := totpCode(secret, time.Now())
	ch := map[string]any{"mfa_token": lout.MFAToken, "code": code, "remember_device": true}
	j, _ := json.Marshal(ch)
	req2, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", bytes.NewReader(j))
	req2.Header.Set("Content-Type", "application/json")
	r2, err := c.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	if r2.StatusCode != 200 {
		b, _ := io.ReadAll(r2.Body)
		r2.Body.Close()
		t.Fatalf("challenge status=%d body=%s", r2.StatusCode, string(b))
	}
	// Capturamos cookies (el http.Client ya las mantiene en jar interno si configurado)
	io.Copy(io.Discard, r2.Body)
	r2.Body.Close()

	// 3) Segundo login: NO debería devolver mfa_required (tokens directos)
	resp2, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("login2 status=%d body=%s", resp2.StatusCode, string(b))
	}
	var out2 struct {
		AccessToken string   `json:"access_token"`
		MFAToken    string   `json:"mfa_token"`
		Required    bool     `json:"mfa_required"`
		AMR         []string `json:"amr"`
	}
	if err := mustJSON(resp2.Body, &out2); err != nil {
		t.Fatal(err)
	}
	if out2.Required || out2.MFAToken != "" {
		t.Fatalf("no se esperaba mfa_required en segundo login (trusted device)")
	}
	if out2.AccessToken == "" {
		t.Fatalf("faltó access token en segundo login")
	}
	// (Opcional) comprobar que amr incluye mfa por el trust
}
