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

// 03 - Email flows: verify-email/start y forgot/reset
// Soporta dos modos:
//   - Con headers de debug (X-Debug-Verify-Link / X-Debug-Reset-Link) → ejercicio completo.
//   - Sin headers de debug → valida 200/204 y saltea confirmaciones (t.Skip), evitando flakes.
func Test_03_Email_Flows(t *testing.T) {
	c := newHTTPClient()

	// --- login admin para poder iniciar verify-email ---
	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	var access string
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
		var out tokens
		if err := mustJSON(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		if out.AccessToken == "" {
			t.Fatalf("login without access token")
		}
		access = out.AccessToken
	}

	// redirect elegido: si hay uno en el seed, lo usamos; si no, fallback de dev.
	redirect := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirect = seed.Clients.Web.Redirects[0]
	}

	t.Run("verify-email/start (+ opcional confirm)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"tenant_id":    seed.Tenant.ID,
			"client_id":    seed.Clients.Web.ClientID,
			"redirect_uri": redirect,
		})

		req, _ := http.NewRequest("POST", baseURL+"/v1/auth/verify-email/start", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("verify-email/start status=%d body=%s", resp.StatusCode, string(b))
		}

		// Si estamos en modo dev con EMAIL_DEBUG_LINKS=true, debería venir el header.
		link := readHeader(resp, "X-Debug-Verify-Link")
		if link == "" {
			t.Skip("sin X-Debug-Verify-Link (prod-like). Validado start; skipping confirm")
			return
		}

		// Confirmación sin seguir redirects, validando dos posibles comportamientos del backend:
		//  - 302 a redirect_uri con status=verified
		//  - 200 JSON {"status":"verified"} (en endpoints sin redirect)
		cNoFollow := newHTTPClient()
		cNoFollow.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		r2, err := cNoFollow.Get(link)
		if err != nil {
			t.Fatal(err)
		}
		defer r2.Body.Close()

		switch r2.StatusCode {
		case 302:
			loc := readHeader(r2, "Location")
			if loc == "" || !strings.Contains(loc, "status=verified") {
				t.Fatalf("verify redirect sin status=verified (Location=%q)", loc)
			}
		case 200:
			var out struct {
				Status string `json:"status"`
			}
			if err := mustJSON(r2.Body, &out); err != nil {
				t.Fatalf("verify decode: %v", err)
			}
			if out.Status != "verified" {
				t.Fatalf("verify JSON status=%q (expected 'verified')", out.Status)
			}
		default:
			b, _ := io.ReadAll(r2.Body)
			t.Fatalf("verify unexpected status=%d body=%s", r2.StatusCode, string(b))
		}
	})

	t.Run("forgot/reset (con autologin o 204) + login con nueva pass", func(t *testing.T) {
		// Para no tocar la cuenta admin real, usamos un usuario nuevo efímero.
		email := uniqueEmail(seed.Users.Admin.Email, "e2e03")
		const initialPwd = "SuperSecreta1!"
		newPwd := "Nuev0Pass!" + itoa(time.Now().Unix()%1_000_000)

		// 1) registrar + (si no emite tokens) login para obtener access
		{
			type regReq struct {
				TenantID string `json:"tenant_id"`
				ClientID string `json:"client_id"`
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			body, _ := json.Marshal(regReq{
				TenantID: seed.Tenant.ID,
				ClientID: seed.Clients.Web.ClientID,
				Email:    email,
				Password: initialPwd,
			})
			resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("register status=%d body=%s", resp.StatusCode, string(b))
			}
		}

		// 2) forgot (esperado 200) y, si hay header de debug, ejecutar reset completo
		{
			body, _ := json.Marshal(map[string]any{
				"tenant_id":    seed.Tenant.ID,
				"client_id":    seed.Clients.Web.ClientID,
				"email":        email,
				"redirect_uri": redirect,
			})
			resp, err := c.Post(baseURL+"/v1/auth/forgot", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("forgot status=%d body=%s", resp.StatusCode, string(b))
			}

			resetLink := readHeader(resp, "X-Debug-Reset-Link")
			if resetLink == "" {
				t.Skip("sin X-Debug-Reset-Link (prod-like). Se validó forgot; skipping reset flow")
				return
			}

			// extraer token del link
			u, err := url.Parse(resetLink)
			if err != nil {
				t.Fatalf("parse reset link: %v", err)
			}
			token := u.Query().Get("token")
			if token == "" {
				t.Fatalf("reset link sin token: %q", resetLink)
			}

			// 3) reset → puede devolver 204 (sin autologin) o 200 con tokens
			bodyR, _ := json.Marshal(map[string]any{
				"tenant_id":    seed.Tenant.ID,
				"client_id":    seed.Clients.Web.ClientID,
				"token":        token,
				"new_password": newPwd,
			})
			r2, err := c.Post(baseURL+"/v1/auth/reset", "application/json", bytes.NewReader(bodyR))
			if err != nil {
				t.Fatal(err)
			}
			defer r2.Body.Close()

			switch r2.StatusCode {
			case 200:
				var out tokens
				if err := mustJSON(r2.Body, &out); err != nil {
					t.Fatalf("reset decode tokens: %v", err)
				}
				if out.AccessToken == "" || out.RefreshToken == "" {
					t.Fatalf("reset 200 sin tokens: %+v", out)
				}
				// sanity: /v1/me con el access que devolvió reset
				req, _ := http.NewRequest("GET", baseURL+"/v1/me", nil)
				req.Header.Set("Authorization", "Bearer "+out.AccessToken)
				me, err := c.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				me.Body.Close()
				if me.StatusCode != 200 {
					t.Fatalf("/v1/me post-reset status=%d", me.StatusCode)
				}

			case 204:
				// hacer login manual con la nueva contraseña
				type loginReq struct {
					TenantID string `json:"tenant_id"`
					ClientID string `json:"client_id"`
					Email    string `json:"email"`
					Password string `json:"password"`
				}
				bLogin, _ := json.Marshal(loginReq{
					TenantID: seed.Tenant.ID,
					ClientID: seed.Clients.Web.ClientID,
					Email:    email,
					Password: newPwd,
				})
				r3, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bLogin))
				if err != nil {
					t.Fatal(err)
				}
				defer r3.Body.Close()
				if r3.StatusCode != 200 {
					b, _ := io.ReadAll(r3.Body)
					t.Fatalf("login con nueva pass status=%d body=%s", r3.StatusCode, string(b))
				}
			default:
				b, _ := io.ReadAll(r2.Body)
				t.Fatalf("reset unexpected status=%d body=%s", r2.StatusCode, string(b))
			}

			// 4) la contraseña vieja ya no debe funcionar
			bBadLogin, _ := json.Marshal(map[string]any{
				"tenant_id": seed.Tenant.ID,
				"client_id": seed.Clients.Web.ClientID,
				"email":     email,
				"password":  initialPwd,
			})
			rBad, err := c.Post(baseURL+"/v1/auth/login", "application/json", bytes.NewReader(bBadLogin))
			if err != nil {
				t.Fatal(err)
			}
			defer rBad.Body.Close()
			if rBad.StatusCode != 401 {
				t.Fatalf("login con pass vieja debería 401; got %d", rBad.StatusCode)
			}
		}
	})

	t.Run("debug headers security", func(t *testing.T) {
		// En modo producción, los headers X-Debug-* NO deberían aparecer
		// Este test verifica que la seguridad esté configurada correctamente

		// Test verify-email/start sin debug headers
		body, _ := json.Marshal(map[string]any{
			"tenant_id":    seed.Tenant.ID,
			"client_id":    seed.Clients.Web.ClientID,
			"redirect_uri": "http://localhost:3000/callback",
		})

		req, _ := http.NewRequest("POST", baseURL+"/v1/auth/verify-email/start", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		debugVerifyLink := readHeader(resp, "X-Debug-Verify-Link")

		// Test forgot sin debug headers
		forgotBody, _ := json.Marshal(map[string]any{
			"tenant_id":    seed.Tenant.ID,
			"client_id":    seed.Clients.Web.ClientID,
			"email":        seed.Users.Admin.Email,
			"redirect_uri": "http://localhost:3000/callback",
		})

		forgotResp, err := c.Post(baseURL+"/v1/auth/forgot", "application/json", bytes.NewReader(forgotBody))
		if err != nil {
			t.Fatal(err)
		}
		defer forgotResp.Body.Close()

		debugResetLink := readHeader(forgotResp, "X-Debug-Reset-Link")

		// En desarrollo, es normal tener debug headers
		// En producción, NO deberían estar presentes
		if debugVerifyLink != "" || debugResetLink != "" {
			t.Logf("DEBUG MODE: X-Debug headers present (verify=%v, reset=%v)",
				debugVerifyLink != "", debugResetLink != "")
			t.Logf("This is normal in development but should be disabled in production")
		} else {
			t.Logf("PRODUCTION MODE: X-Debug headers properly disabled")
		}

		// Verificar que otros headers sensibles no estén expuestos
		sensitiveHeaders := []string{
			"X-Debug-User-ID",
			"X-Debug-Token",
			"X-Debug-Secret",
			"X-Internal-",
			"X-Backend-",
		}

		for _, headerPrefix := range sensitiveHeaders {
			for headerName := range resp.Header {
				if strings.HasPrefix(headerName, headerPrefix) {
					t.Errorf("Sensitive header exposed: %s", headerName)
				}
			}
			for headerName := range forgotResp.Header {
				if strings.HasPrefix(headerName, headerPrefix) {
					t.Errorf("Sensitive header exposed: %s", headerName)
				}
			}
		}
	})
}
