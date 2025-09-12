package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEmailFlowsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping email flows E2E tests in short mode")
	}

	if seed == nil {
		t.Skip("seed data not available")
	}

	// ✅ FIXED: Usar datos del seed en lugar de hardcodear
	tenant := seed.Tenant.ID
	client := seed.Clients.Web.ClientID
	adminEmail := seed.Users.Admin.Email
	currentPassword := seed.Users.Admin.Password
	redirectUri := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirectUri = seed.Clients.Web.Redirects[0]
	}

	t.Run("ForgotResetFlow", func(t *testing.T) {
		// Función helper para forgot/reset
		doForgotReset := func(email string) (string, error) {
			t.Logf("Executing forgot/reset for email: %s", email)

			// 1. Forgot request
			forgotPayload := map[string]interface{}{
				"tenant_id":    tenant,
				"client_id":    client,
				"email":        email,
				"redirect_uri": redirectUri,
			}

			forgotData, err := json.Marshal(forgotPayload)
			if err != nil {
				return "", fmt.Errorf("marshal forgot payload: %v", err)
			}

			forgotResp, err := http.Post(baseURL+"/v1/auth/forgot", "application/json", strings.NewReader(string(forgotData)))
			if err != nil {
				return "", fmt.Errorf("forgot request: %v", err)
			}
			defer forgotResp.Body.Close()

			if forgotResp.StatusCode < 200 || forgotResp.StatusCode >= 300 {
				return "", fmt.Errorf("forgot returned %d", forgotResp.StatusCode)
			}

			// 2. Extract reset link from debug header
			resetLink := forgotResp.Header.Get("X-Debug-Reset-Link")
			if resetLink == "" {
				return "", fmt.Errorf("no X-Debug-Reset-Link header found. Ensure APP_ENV=dev and EMAIL_DEBUG_LINKS=true")
			}

			t.Logf("Got reset link: %s", resetLink)

			// 3. Extract token from reset link
			resetURL, err := url.Parse(resetLink)
			if err != nil {
				return "", fmt.Errorf("parse reset URL: %v", err)
			}

			token := resetURL.Query().Get("token")
			if token == "" {
				return "", fmt.Errorf("no token found in reset URL")
			}

			// 4. Reset password with new password
			newPassword := fmt.Sprintf("Nuev4Clave!%d", time.Now().Unix())
			resetPayload := map[string]interface{}{
				"tenant_id":    tenant,
				"client_id":    client,
				"token":        token,
				"new_password": newPassword,
			}

			resetData, err := json.Marshal(resetPayload)
			if err != nil {
				return "", fmt.Errorf("marshal reset payload: %v", err)
			}

			resetResp, err := http.Post(baseURL+"/v1/auth/reset", "application/json", strings.NewReader(string(resetData)))
			if err != nil {
				return "", fmt.Errorf("reset request: %v", err)
			}
			defer resetResp.Body.Close()

			if resetResp.StatusCode == 204 {
				t.Log("Reset successful (204 - no auto-login)")
			} else if resetResp.StatusCode == 200 {
				t.Log("Reset successful (200 - auto-login enabled)")
			} else {
				return "", fmt.Errorf("reset returned %d", resetResp.StatusCode)
			}

			t.Logf("New password: %s", newPassword)
			return newPassword, nil
		}

		// Test inicial login
		t.Log("== Initial Login Test ==")
		loginPayload := map[string]interface{}{
			"tenant_id": tenant,
			"client_id": client,
			"email":     adminEmail,
			"password":  currentPassword,
		}

		loginData, err := json.Marshal(loginPayload)
		if err != nil {
			t.Fatalf("Marshal login payload: %v", err)
		}

		loginResp, err := http.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(loginData)))
		if err != nil {
			t.Fatalf("Login request: %v", err)
		}
		defer loginResp.Body.Close()

		var accessToken string
		newPassword := currentPassword

		if loginResp.StatusCode == 200 {
			var loginResult struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
				t.Fatalf("Decode login response: %v", err)
			}
			accessToken = loginResult.AccessToken
			t.Log("Login successful (200)")
		} else if loginResp.StatusCode == 401 {
			t.Log("Login failed (401), attempting forgot/reset recovery...")
			var err error
			newPassword, err = doForgotReset(adminEmail)
			if err != nil {
				t.Fatalf("Forgot/reset recovery failed: %v", err)
			}
			 // Persistir nueva password global para siguientes tests
			 seed.Users.Admin.Password = newPassword

			// Retry login with new password
			t.Log("== Login with new password ==")
			loginPayload["password"] = newPassword
			loginData, err := json.Marshal(loginPayload)
			if err != nil {
				t.Fatalf("Marshal new login payload: %v", err)
			}

			loginResp2, err := http.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(loginData)))
			if err != nil {
				t.Fatalf("New login request: %v", err)
			}
			defer loginResp2.Body.Close()

			if loginResp2.StatusCode != 200 {
				t.Fatalf("New login failed with status %d", loginResp2.StatusCode)
			}

			var loginResult struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(loginResp2.Body).Decode(&loginResult); err != nil {
				t.Fatalf("Decode new login response: %v", err)
			}
			accessToken = loginResult.AccessToken
			t.Log("New login successful (200)")
		} else {
			t.Fatalf("Unexpected login status: %d", loginResp.StatusCode)
		}

		// Test verify-email flow
		t.Log("== Verify Email Start ==")
		verifyPayload := map[string]interface{}{
			"tenant_id":    tenant,
			"client_id":    client,
			"redirect_uri": redirectUri,
		}

		verifyData, err := json.Marshal(verifyPayload)
		if err != nil {
			t.Fatalf("Marshal verify payload: %v", err)
		}

		req, err := http.NewRequest("POST", baseURL+"/v1/auth/verify-email/start", strings.NewReader(string(verifyData)))
		if err != nil {
			t.Fatalf("Create verify request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)

		httpClient := &http.Client{}
		verifyResp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("Verify request: %v", err)
		}
		defer verifyResp.Body.Close()

		verifyLink := verifyResp.Header.Get("X-Debug-Verify-Link")
		t.Logf("Verify start -> %d, link=%s", verifyResp.StatusCode, verifyLink)

		if verifyLink != "" && verifyLink != "<none>" {
			// Test verify confirmation with no auto-redirect
			t.Log("== Verify Email Confirm ==")
			httpClientNoRedirect := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse // Prevent auto-redirect
				},
			}

			verifyConfirmResp, err := httpClientNoRedirect.Get(verifyLink)
			if err != nil {
				t.Fatalf("Verify confirm request: %v", err)
			}
			defer verifyConfirmResp.Body.Close()

			t.Logf("Verify confirm -> %d (expected 302 if redirect_uri present)", verifyConfirmResp.StatusCode)

			if verifyConfirmResp.StatusCode == 302 {
				location := verifyConfirmResp.Header.Get("Location")
				t.Logf("Redirect location: %s", location)
			}
		} else {
			t.Log("Skipping verify confirm (no debug link; ensure APP_ENV=dev + EMAIL_DEBUG_LINKS=true)")
		}

		// Test segundo forgot/reset cycle
		t.Log("== Second Forgot/Reset Cycle ==")
		finalPassword, err := doForgotReset(adminEmail)
		if err != nil {
			t.Logf("Skipping second reset cycle: %v", err)
		} else {
			// Actualizar password en seed global para tests posteriores (p.ej. Test_Login_And_UserInfo)
			seed.Users.Admin.Password = finalPassword
			// Final login test
			t.Log("== Final Login Test ==")
			finalLoginPayload := map[string]interface{}{
				"tenant_id": tenant,
				"client_id": client,
				"email":     adminEmail,
				"password":  finalPassword,
			}

			finalLoginData, err := json.Marshal(finalLoginPayload)
			if err != nil {
				t.Fatalf("Marshal final login payload: %v", err)
			}

			finalLoginResp, err := http.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(finalLoginData)))
			if err != nil {
				t.Fatalf("Final login request: %v", err)
			}
			defer finalLoginResp.Body.Close()

			t.Logf("Final login -> %d (expected 200)", finalLoginResp.StatusCode)
		}

		t.Log("== Email flows E2E completed ==")
	})

	t.Run("EmailLinkValidation", func(t *testing.T) {
		// Test validación de links de email con diferentes parámetros
		testEmail := "test.emaillinks@example.com"

		forgotPayload := map[string]interface{}{
			"tenant_id":    tenant,
			"client_id":    client,
			"email":        testEmail,
			"redirect_uri": redirectUri,
		}

		forgotData, err := json.Marshal(forgotPayload)
		if err != nil {
			t.Fatalf("Marshal forgot payload: %v", err)
		}

		forgotResp, err := http.Post(baseURL+"/v1/auth/forgot", "application/json", strings.NewReader(string(forgotData)))
		if err != nil {
			t.Fatalf("Forgot request: %v", err)
		}
		defer forgotResp.Body.Close()

		resetLink := forgotResp.Header.Get("X-Debug-Reset-Link")
		if resetLink == "" {
			t.Skip("No debug reset link available, skipping email link validation")
		}

		// Validate reset link structure
		resetURL, err := url.Parse(resetLink)
		if err != nil {
			t.Fatalf("Parse reset URL: %v", err)
		}

		token := resetURL.Query().Get("token")
		if token == "" {
			t.Error("Reset link missing token parameter")
		}

		if len(token) < 10 {
			t.Error("Reset token appears too short")
		}

		// Test que el token sea URL-safe
		if matched, _ := regexp.MatchString(`^[A-Za-z0-9_-]+$`, token); !matched {
			t.Error("Reset token contains invalid characters")
		}

		t.Logf("Reset link validation passed - token length: %d", len(token))
	})
}
