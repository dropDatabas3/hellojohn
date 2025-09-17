package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"
)

// 17 - MFA Negativos: reuse recovery, reuse TOTP (misma ventana), token expirado (pendiente)
func Test_17_MFA_Negative(t *testing.T) {
	if seed == nil || seed.Users.MFA.Email == "" {
		t.Skip("seed.Users.MFA vacío")
	}
	secret := totpSecretFromSeed(t)
	c := newHTTPClient()

	loginReq := map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.MFA.Email,
		"password":  seed.Users.MFA.Password,
	}
	bLogin, _ := json.Marshal(loginReq)
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bLogin))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("login status=%d body=%s", resp.StatusCode, string(body))
	}
	var lr struct {
		MFAToken string `json:"mfa_token"`
		Required bool   `json:"mfa_required"`
	}
	if err := mustJSON(resp.Body, &lr); err != nil {
		t.Fatal(err)
	}
	if !lr.Required || lr.MFAToken == "" {
		t.Fatalf("se esperaba mfa_required")
	}

	// Recovery reuse test (buscar un recovery no usado; otros tests pueden haber consumido algunos)
	if len(seed.Users.MFA.Recovery) > 0 {
		var usedRecovery string
		for _, rc := range seed.Users.MFA.Recovery {
			use1 := map[string]any{"mfa_token": lr.MFAToken, "recovery": rc}
			j1, _ := json.Marshal(use1)
			r1, err := c.Post(baseURL+"/v1/mfa/totp/challenge", "application/json", bytes.NewReader(j1))
			if err != nil {
				t.Fatal(err)
			}
			if r1.StatusCode == 200 {
				io.Copy(io.Discard, r1.Body)
				r1.Body.Close()
				usedRecovery = rc
				break
			}
			// consumir body y seguir probando el próximo
			io.Copy(io.Discard, r1.Body)
			r1.Body.Close()
		}
		if usedRecovery == "" {
			t.Skip("no quedó ningún recovery code usable (otro test los consumió)")
		}

		// Nuevo login para reuso del MISMO recovery (debe fallar)
		resp2, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bLogin))
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()
		var lr2 struct {
			MFAToken string `json:"mfa_token"`
			Required bool   `json:"mfa_required"`
		}
		if err := mustJSON(resp2.Body, &lr2); err != nil {
			t.Fatal(err)
		}
		if !lr2.Required || lr2.MFAToken == "" {
			t.Fatalf("segundo login debía pedir MFA")
		}

		reuse := map[string]any{"mfa_token": lr2.MFAToken, "recovery": usedRecovery}
		j2, _ := json.Marshal(reuse)
		r2, err := c.Post(baseURL+"/v1/mfa/totp/challenge", "application/json", bytes.NewReader(j2))
		if err != nil {
			t.Fatal(err)
		}
		if r2.StatusCode == 200 {
			body, _ := io.ReadAll(r2.Body)
			r2.Body.Close()
			t.Fatalf("reuse recovery aceptado body=%s", string(body))
		}
		io.Copy(io.Discard, r2.Body)
		r2.Body.Close()

		// Prepara nuevo login para test TOTP reuse con token fresco
		resp, err = c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bLogin))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if err := mustJSON(resp.Body, &lr); err != nil {
			t.Fatal(err)
		}
		if !lr.Required || lr.MFAToken == "" {
			t.Fatalf("tercer login debía pedir MFA")
		}
	}

	// TOTP válido una vez (evitar usar un código cerca del límite de la ventana)
	now := time.Now()
	// si estamos en los últimos 2s de la ventana actual, esperamos al próximo tick para evitar falsos negativos
	if now.Unix()%30 >= 28 {
		time.Sleep(time.Duration(31-now.Unix()%30) * time.Second)
		now = time.Now()
	}
	code := totpCode(secret, now)
	okPayload := map[string]any{"mfa_token": lr.MFAToken, "code": code}
	j3, _ := json.Marshal(okPayload)
	r3, err := c.Post(baseURL+"/v1/mfa/totp/challenge", "application/json", bytes.NewReader(j3))
	if err != nil {
		t.Fatal(err)
	}
	if r3.StatusCode != 200 {
		body, _ := io.ReadAll(r3.Body)
		r3.Body.Close()
		// Si falló por ventana (código inválido posiblemente porque last_used_at == ventana actual), esperar próximo tick y reintentar una vez
		// Esperar hasta el comienzo de la próxima ventana de 30s (+1s de colchón)
		now2 := time.Now()
		wait := time.Duration(31-(now2.Unix()%30)) * time.Second
		if wait > 0 && wait <= 31*time.Second {
			time.Sleep(wait)
		}
		// Nuevo código y reintento
		codeRetry := totpCode(secret, time.Now())
		okPayloadRetry := map[string]any{"mfa_token": lr.MFAToken, "code": codeRetry}
		jRetry, _ := json.Marshal(okPayloadRetry)
		r3b, err2 := c.Post(baseURL+"/v1/mfa/totp/challenge", "application/json", bytes.NewReader(jRetry))
		if err2 != nil {
			t.Fatal(err2)
		}
		if r3b.StatusCode != 200 {
			body2, _ := io.ReadAll(r3b.Body)
			r3b.Body.Close()
			t.Fatalf("totp inicial (retry) status=%d body=%s; first=%s", r3b.StatusCode, string(body2), string(body))
		}
		io.Copy(io.Discard, r3b.Body)
		r3b.Body.Close()
	} else {
		io.Copy(io.Discard, r3.Body)
		r3.Body.Close()
	}

	// Nuevo login y reusar mismo TOTP (misma ventana) => debe fallar
	resp4, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bLogin))
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	var lr4 struct {
		MFAToken string `json:"mfa_token"`
		Required bool   `json:"mfa_required"`
	}
	if err := mustJSON(resp4.Body, &lr4); err != nil {
		t.Fatal(err)
	}
	if !lr4.Required || lr4.MFAToken == "" {
		t.Fatalf("cuarto login debía pedir MFA")
	}
	reuseTotp := map[string]any{"mfa_token": lr4.MFAToken, "code": code}
	j4, _ := json.Marshal(reuseTotp)
	r4, err := c.Post(baseURL+"/v1/mfa/totp/challenge", "application/json", bytes.NewReader(j4))
	if err != nil {
		t.Fatal(err)
	}
	if r4.StatusCode == 200 {
		body, _ := io.ReadAll(r4.Body)
		r4.Body.Close()
		t.Fatalf("reuse TOTP aceptado body=%s", string(body))
	}
	io.Copy(io.Discard, r4.Body)
	r4.Body.Close()

	// Token expirado (pendiente): requeriría manipular TTL o reloj; se omite para evitar test lento.

	// Invalid/Expired mfa_token: should fail with 404/400 and never issue tokens
	{
		bad := map[string]any{
			"mfa_token": "deadbeefdeadbeefdeadbeefdeadbeef",
			"code":      "000000",
		}
		jb, _ := json.Marshal(bad)
		rb, err := c.Post(baseURL+"/v1/mfa/totp/challenge", "application/json", bytes.NewReader(jb))
		if err != nil {
			t.Fatal(err)
		}
		defer rb.Body.Close()
		if rb.StatusCode != 404 && rb.StatusCode != 400 {
			body, _ := io.ReadAll(rb.Body)
			t.Fatalf("expected 404/400 for invalid/expired mfa_token, got %d body=%s", rb.StatusCode, string(body))
		}
		io.Copy(io.Discard, rb.Body)
	}
}
