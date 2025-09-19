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

// 06 - MFA TOTP: gating, código inválido y éxito con TOTP válido.
// Precondición: seed.Users.MFA tiene secret base32 (o un otpauth_url) y el
// usuario está configurado para requerir MFA al hacer login.
func Test_06_MFA_TOTP(t *testing.T) {
	if seed == nil || seed.Users.MFA.Email == "" {
		t.Skip("seed.Users.MFA vacío; skipping")
	}
	secret := totpSecretFromSeed(t)
	c := newHTTPClient()

	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type loginResp struct {
		MFAToken string `json:"mfa_token"`
		Required bool   `json:"mfa_required"`
		// si el server permite autologin sin MFA (no debería): tokens
		AccessToken  string `json:"access_token,omitempty"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	// 1) Login: debe gatillar MFA con mfa_token
	lr := loginReq{
		TenantID: seed.Tenant.ID,
		ClientID: seed.Clients.Web.ClientID,
		Email:    seed.Users.MFA.Email,
		Password: seed.Users.MFA.Password,
	}
	body, _ := json.Marshal(lr)
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login status=%d body=%s", resp.StatusCode, string(b))
	}
	var lout loginResp
	if err := mustJSON(resp.Body, &lout); err != nil {
		t.Fatal(err)
	}
	if !lout.Required || lout.MFAToken == "" {
		t.Fatalf("se esperaba mfa_required con mfa_token; got %+v", lout)
	}

	// 2) Challenge con código inválido → 400/401
	{
		bad := map[string]any{
			"mfa_token": lout.MFAToken,
			"code":      "000000",
		}
		j, _ := json.Marshal(bad)
		req, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", bytes.NewReader(j))
		req.Header.Set("Content-Type", "application/json")
		r2, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r2.Body)
		r2.Body.Close()
		if r2.StatusCode != 400 && r2.StatusCode != 401 {
			t.Fatalf("bad TOTP expected 400/401, got %d", r2.StatusCode)
		}
	}

	// 3) Challenge con TOTP válido → 200 + tokens
	type chResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	var out chResp
	{
		// idealmente dentro de la misma ventana de 30s
		code := totpCode(secret, time.Now())
		ok := map[string]any{
			"mfa_token":        lout.MFAToken,
			"code":             code,
			"remember_device":  true, // opcional (no afirmamos comportamiento)
			"client_id":        seed.Clients.Web.ClientID,
			"tenant_id":        seed.Tenant.ID,
			"token_type_hint":  "access_token", // ignorado si no aplica
			"requested_scopes": []string{"openid", "email"},
		}
		j, _ := json.Marshal(ok)
		req, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", bytes.NewReader(j))
		req.Header.Set("Content-Type", "application/json")
		r3, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r3.Body.Close()
		if r3.StatusCode != 200 {
			b, _ := io.ReadAll(r3.Body)
			t.Fatalf("challenge (valid totp) status=%d body=%s", r3.StatusCode, string(b))
		}
		if err := mustJSON(r3.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.AccessToken == "" || out.RefreshToken == "" || out.ExpiresIn <= 0 {
			t.Fatalf("tokens inválidos en challenge: %+v", out)
		}
	}

	// 4) /userinfo con el access devuelto
	{
		req, _ := http.NewRequest("GET", baseURL+"/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+out.AccessToken)
		u, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer u.Body.Close()
		if u.StatusCode != 200 {
			b, _ := io.ReadAll(u.Body)
			t.Fatalf("userinfo status=%d body=%s", u.StatusCode, string(b))
		}
	}
}

// totpSecretFromSeed: usa seed.Users.MFA.TOTPBase32 si existe; de lo contrario
// intenta extraer ?secret= de OTPAuthURL. Fail-fast si no se encuentra.
func totpSecretFromSeed(t *testing.T) string {
	if s := strings.TrimSpace(seed.Users.MFA.TOTPBase32); s != "" {
		return s
	}
	if raw := strings.TrimSpace(seed.Users.MFA.OTPAuthURL); raw != "" {
		u, err := url.Parse(raw)
		if err == nil {
			if sec := u.Query().Get("secret"); sec != "" {
				return sec
			}
		}
	}
	t.Fatalf("seed MFA sin secret base32 ni otpauth_url válido")
	return ""
}
