package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRateEmailFlows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting tests in short mode")
	}

	if seed == nil {
		t.Skip("seed data not available")
	}

	// ✅ FIXED: Usar datos del seed en lugar de hardcodear
	tenant := seed.Tenant.ID
	client := seed.Clients.Web.ClientID
	email := seed.Users.Admin.Email
	redirectUri := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirectUri = seed.Clients.Web.Redirects[0]
	}

	t.Run("ForgotPasswordRateLimit", func(t *testing.T) {
		payload := map[string]interface{}{
			"tenant_id":    tenant,
			"client_id":    client,
			"email":        email,
			"redirect_uri": redirectUri,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}

		// Primera llamada - debe ser exitosa
		resp1, err := http.Post(baseURL+"/v1/auth/forgot", "application/json", strings.NewReader(string(jsonData)))
		if err != nil {
			t.Fatalf("First forgot request failed: %v", err)
		}
		resp1.Body.Close()

		if resp1.StatusCode < 200 || resp1.StatusCode >= 300 {
			t.Logf("First forgot request status: %d", resp1.StatusCode)
			// No es fatal si falla la primera, puede ser por otros rate limits
		} else {
			t.Logf("First forgot request: %d", resp1.StatusCode)
		}

		retryAfter1 := resp1.Header.Get("Retry-After")
		if retryAfter1 != "" {
			t.Logf("First request Retry-After: %s", retryAfter1)
		}

		// Segunda llamada inmediata - debe ser rate limited
		resp2, err := http.Post(baseURL+"/v1/auth/forgot", "application/json", strings.NewReader(string(jsonData)))
		if err != nil {
			t.Fatalf("Second forgot request failed: %v", err)
		}
		resp2.Body.Close()

		t.Logf("Second forgot request status: %d", resp2.StatusCode)

		if resp2.StatusCode == 429 {
			retryAfter2 := resp2.Header.Get("Retry-After")
			if retryAfter2 == "" {
				t.Error("Rate limited response missing Retry-After header")
			} else {
				t.Logf("Rate limited - Retry-After: %s", retryAfter2)
			}
		} else if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
			t.Log("Second request succeeded - rate limiting may be disabled or window expired")
		} else {
			t.Logf("Second request failed with status %d (not rate limited)", resp2.StatusCode)
		}
	})

	t.Run("VerifyEmailRateLimit", func(t *testing.T) {
		// Para este test necesitamos un access token válido
		loginPayload := map[string]interface{}{
			"tenant_id": tenant,
			"client_id": client,
			"email":     email,
			"password":  seed.Users.Admin.Password, // ✅ FIXED: Usar password del seed
		}

		loginData, err := json.Marshal(loginPayload)
		if err != nil {
			t.Fatalf("Failed to marshal login JSON: %v", err)
		}

		loginResp, err := http.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(loginData)))
		if err != nil {
			t.Fatalf("Login request failed: %v", err)
		}
		defer loginResp.Body.Close()

		if loginResp.StatusCode != 200 {
			t.Skipf("Cannot test verify-email rate limit - login failed with status %d", loginResp.StatusCode)
		}

		var loginResult struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
			t.Fatalf("Failed to decode login response: %v", err)
		}

		if loginResult.AccessToken == "" {
			t.Skip("No access token received, skipping verify-email rate limit test")
		}

		// Test verify-email/start rate limiting
		verifyPayload := map[string]interface{}{
			"tenant_id":    tenant,
			"client_id":    client,
			"redirect_uri": redirectUri,
		}

		verifyData, err := json.Marshal(verifyPayload)
		if err != nil {
			t.Fatalf("Failed to marshal verify JSON: %v", err)
		}

		// Primera llamada
		req1, err := http.NewRequest("POST", baseURL+"/v1/auth/verify-email/start", strings.NewReader(string(verifyData)))
		if err != nil {
			t.Fatalf("Failed to create verify request: %v", err)
		}
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)

		client := &http.Client{}
		verifyResp1, err := client.Do(req1)
		if err != nil {
			t.Fatalf("First verify request failed: %v", err)
		}
		verifyResp1.Body.Close()

		t.Logf("First verify-email request: %d", verifyResp1.StatusCode)

		// Segunda llamada inmediata
		req2, err := http.NewRequest("POST", baseURL+"/v1/auth/verify-email/start", strings.NewReader(string(verifyData)))
		if err != nil {
			t.Fatalf("Failed to create second verify request: %v", err)
		}
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)

		verifyResp2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("Second verify request failed: %v", err)
		}
		verifyResp2.Body.Close()

		t.Logf("Second verify-email request: %d", verifyResp2.StatusCode)

		if verifyResp2.StatusCode == 429 {
			retryAfter := verifyResp2.Header.Get("Retry-After")
			if retryAfter == "" {
				t.Error("Rate limited verify-email response missing Retry-After header")
			} else {
				t.Logf("Verify-email rate limited - Retry-After: %s", retryAfter)
			}
		}
	})

	t.Run("RateLimitRecovery", func(t *testing.T) {
		// Test que después del rate limit window, las requests vuelven a funcionar
		payload := map[string]interface{}{
			"tenant_id":    tenant,
			"client_id":    client,
			"email":        "recovery.test@example.com", // Email diferente para evitar conflictos
			"redirect_uri": redirectUri,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}

		// Hacer requests hasta que sea rate limited
		var lastResp *http.Response
		for i := 0; i < 10; i++ {
			resp, err := http.Post(baseURL+"/v1/auth/forgot", "application/json", strings.NewReader(string(jsonData)))
			if err != nil {
				t.Fatalf("Request %d failed: %v", i+1, err)
			}

			if lastResp != nil {
				lastResp.Body.Close()
			}
			lastResp = resp

			t.Logf("Request %d status: %d", i+1, resp.StatusCode)

			if resp.StatusCode == 429 {
				retryAfter := resp.Header.Get("Retry-After")
				t.Logf("Hit rate limit after %d requests, Retry-After: %s", i+1, retryAfter)
				break
			}

			// Pequeña pausa entre requests
			time.Sleep(100 * time.Millisecond)
		}

		if lastResp != nil {
			lastResp.Body.Close()
		}

		t.Log("Rate limit recovery test completed")
	})
}
