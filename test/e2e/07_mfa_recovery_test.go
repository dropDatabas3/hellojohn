package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// 07 - MFA Recovery: desafío con código de recuperación y reuso bloqueado.
// Precondición: seed.Users.MFA.Recovery debe traer al menos 1 código.
func Test_07_MFA_Recovery(t *testing.T) {
	if seed == nil || len(seed.Users.MFA.Recovery) == 0 {
		t.Skip("seed MFA sin recovery codes; skipping")
	}
	c := newHTTPClient()

	// 1) Login gatillando MFA
	lr := map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.MFA.Email,
		"password":  seed.Users.MFA.Password,
	}
	b, _ := json.Marshal(lr)
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
	var lout struct {
		MFAToken string `json:"mfa_token"`
		Required bool   `json:"mfa_required"`
	}
	if err := mustJSON(resp.Body, &lout); err != nil {
		t.Fatal(err)
	}
	if !lout.Required || lout.MFAToken == "" {
		t.Fatalf("se esperaba mfa_required con mfa_token; got %+v", lout)
	}

	// 2) Challenge sin code ni recovery -> 400
	{
		reqBody := map[string]any{
			"mfa_token": lout.MFAToken,
			// ni "code" ni "recovery"
		}
		j, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", bytes.NewReader(j))
		req.Header.Set("Content-Type", "application/json")
		r, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 400 {
			t.Fatalf("expected 400 when missing code/recovery, got %d", r.StatusCode)
		}
	}

	// 3) Challenge con recovery válido -> 200 + tokens
	first := seed.Users.MFA.Recovery[0]
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	{
		reqBody := map[string]any{
			"mfa_token":       lout.MFAToken,
			"recovery":        first,
			"remember_device": true, // opcional
		}
		j, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", bytes.NewReader(j))
		req.Header.Set("Content-Type", "application/json")
		r, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			buf, _ := io.ReadAll(r.Body)
			t.Fatalf("recovery challenge status=%d body=%s", r.StatusCode, string(buf))
		}
		if err := mustJSON(r.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.AccessToken == "" || out.RefreshToken == "" || out.ExpiresIn <= 0 {
			t.Fatalf("tokens inválidos: %+v", out)
		}
	}

	// 4) Reusar el MISMO recovery -> debe fallar (consumido)
	{
		reqBody := map[string]any{
			"mfa_token":       lout.MFAToken,
			"recovery":        first, // reuso
			"remember_device": false,
		}
		j, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", bytes.NewReader(j))
		req.Header.Set("Content-Type", "application/json")
		r, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode == 200 {
			t.Fatalf("reusing recovery code should fail")
		}
	}
}
