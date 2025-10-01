package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

// 27 - CSRF en endpoints con cookie (doble-submit: header + cookie)
// DoD: 403 sin token CSRF; 2xx con token válido y cookie de sesión seteada.
func Test_27_CSRF_Cookie_Flows(t *testing.T) {
	// Habilitar este test sólo cuando el backend enforces CSRF en /v1/session/login
	// Setear CSRF_COOKIE_ENFORCED=true|1 para activar.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("CSRF_COOKIE_ENFORCED"))); v == "" || v == "0" || v == "false" {
		t.Skip("CSRF cookie enforcement not enabled; set CSRF_COOKIE_ENFORCED=1 to run")
	}

	if seed == nil {
		t.Skip("no seed data; skipping")
	}

	csrfHeader := os.Getenv("CSRF_HEADER_NAME")
	if csrfHeader == "" {
		csrfHeader = "X-CSRF-Token"
	}
	csrfCookie := os.Getenv("CSRF_COOKIE_NAME")
	if csrfCookie == "" {
		csrfCookie = "csrf_token"
	}

	// Payload de login de sesión
	body, _ := json.Marshal(map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	})

	// Cliente con cookie jar
	c := newHTTPClient()

	t.Run("denied_without_token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/session/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 without CSRF token, got %d", resp.StatusCode)
		}
	})

	t.Run("allowed_with_valid_token", func(t *testing.T) {
		// Doble-submit: seteamos cookie CSRF y header con mismo valor
		token := "test-csrf-token-123"
		u, _ := url.Parse(baseURL)
		c.Jar.SetCookies(u, []*http.Cookie{{
			Name:  csrfCookie,
			Value: token,
			Path:  "/",
		}})

		req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/session/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrfHeader, token)

		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
			t.Fatalf("expected 2xx/302 with valid CSRF token, got %d", resp.StatusCode)
		}
		if len(resp.Header["Set-Cookie"]) == 0 {
			t.Fatalf("expected session Set-Cookie on successful login")
		}
	})
}
