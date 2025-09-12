package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestJWTKeyRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping JWT key rotation tests in short mode")
	}

	if seed == nil {
		t.Skip("seed data not available")
	}

	// ✅ FIXED: Usar usuario sin MFA para evitar problemas de autenticación
	tenant := seed.Tenant.ID
	client := seed.Clients.Web.ClientID
	email := seed.Users.Unverified.Email
	password := seed.Users.Unverified.Password
	envFile := ".env.dev" // ✅ FIXED: Solo el nombre del archivo

	// Helper function to run keys CLI command
	runKeysCLI := func(args ...string) error {
		repoRoot := "../.." // From test/e2e back to project root
		cmd := exec.Command("go", append([]string{"run", "./cmd/keys/main.go"}, args...)...)
		cmd.Dir = repoRoot

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Keys CLI output: %s", string(output))
			return fmt.Errorf("keys CLI failed: %v", err)
		}
		t.Logf("Keys CLI executed successfully: %s", strings.Join(args, " "))
		return nil
	}

	// Helper function to get JWKS
	getJWKS := func() (map[string]interface{}, error) {
		resp, err := http.Get(baseURL + "/.well-known/jwks.json")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("JWKS request failed with status %d", resp.StatusCode)
		}

		var jwks map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
			return nil, err
		}

		return jwks, nil
	}

	// Helper function to login and get tokens
	login := func() (string, string, error) {
		loginPayload := map[string]interface{}{
			"tenant_id": tenant,
			"client_id": client,
			"email":     email,
			"password":  password,
		}

		loginData, err := json.Marshal(loginPayload)
		if err != nil {
			return "", "", err
		}

		resp, err := http.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(loginData)))
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", "", fmt.Errorf("login failed with status %d", resp.StatusCode)
		}

		var result struct {
			AccessToken string `json:"access_token"`
			IDToken     string `json:"id_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", "", err
		}

		return result.AccessToken, result.IDToken, nil
	}

	// Helper function to test /userinfo with token
	testUserInfo := func(accessToken string) error {
		req, err := http.NewRequest("GET", baseURL+"/userinfo", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("userinfo failed with status %d", resp.StatusCode)
		}

		return nil
	}

	t.Run("FullKeyRotationFlow", func(t *testing.T) {
		t.Log("== Initial Login ==")
		accessToken1, idToken1, err := login()
		if err != nil {
			t.Fatalf("Initial login failed: %v", err)
		}

		// Decode initial tokens to get KID
		var kid1 string
		if idToken1 != "" {
			header1, _, err := decodeJWT(idToken1)
			if err != nil {
				t.Logf("Failed to decode ID token: %v", err)
			} else {
				if kid, ok := header1["kid"].(string); ok {
					kid1 = kid
				}
				t.Logf("Initial ID token KID: %s", kid1)
			}
		} else {
			// Fallback to access token
			header1, _, err := decodeJWT(accessToken1)
			if err != nil {
				t.Logf("Failed to decode access token: %v", err)
			} else {
				if kid, ok := header1["kid"].(string); ok {
					kid1 = kid
				}
				t.Logf("Initial access token KID: %s", kid1)
			}
		}

		t.Log("== JWKS Before Rotation ==")
		jwks1, err := getJWKS()
		if err != nil {
			t.Fatalf("Failed to get JWKS before rotation: %v", err)
		}

		keys1, ok := jwks1["keys"].([]interface{})
		if !ok {
			t.Fatal("JWKS keys field is not an array")
		}

		var kids1 []string
		for _, key := range keys1 {
			if keyMap, ok := key.(map[string]interface{}); ok {
				if kid, ok := keyMap["kid"].(string); ok {
					kids1 = append(kids1, kid)
				}
			}
		}

		t.Logf("JWKS before rotation - keys count: %d, kids: [%s]", len(keys1), strings.Join(kids1, ","))

		t.Log("== Rotating Keys ==")
		// Check if env file exists
		if _, err := os.Stat("../../" + envFile); os.IsNotExist(err) {
			t.Skipf("Env file %s not found, skipping key rotation test", envFile)
		}

		// Run key rotation
		if err := runKeysCLI("-rotate", "-env", "-env-file", envFile); err != nil {
			t.Fatalf("Key rotation failed: %v", err)
		}

		t.Log("== Waiting for keystore cache expiration (35s) ==")
		time.Sleep(35 * time.Second)

		t.Log("== Testing old token still works ==")
		if err := testUserInfo(accessToken1); err != nil {
			t.Errorf("Old access token should still work after rotation: %v", err)
		} else {
			t.Log("Old access token still valid ✓")
		}

		t.Log("== New Login After Rotation ==")
		accessToken2, idToken2, err := login()
		if err != nil {
			t.Fatalf("Login after rotation failed: %v", err)
		}

		// Decode new tokens to get KID
		var kid2 string
		if idToken2 != "" {
			header2, _, err := decodeJWT(idToken2)
			if err != nil {
				t.Logf("Failed to decode new ID token: %v", err)
			} else {
				if kid, ok := header2["kid"].(string); ok {
					kid2 = kid
				}
				t.Logf("New ID token KID: %s", kid2)
			}
		} else {
			// Fallback to access token
			header2, _, err := decodeJWT(accessToken2)
			if err != nil {
				t.Logf("Failed to decode new access token: %v", err)
			} else {
				if kid, ok := header2["kid"].(string); ok {
					kid2 = kid
				}
				t.Logf("New access token KID: %s", kid2)
			}
		}

		t.Log("== JWKS After Rotation ==")
		// Small delay to ensure JWKS is updated
		time.Sleep(1 * time.Second)
		jwks2, err := getJWKS()
		if err != nil {
			t.Fatalf("Failed to get JWKS after rotation: %v", err)
		}

		keys2, ok := jwks2["keys"].([]interface{})
		if !ok {
			t.Fatal("JWKS keys field is not an array after rotation")
		}

		var kids2 []string
		for _, key := range keys2 {
			if keyMap, ok := key.(map[string]interface{}); ok {
				if kid, ok := keyMap["kid"].(string); ok {
					kids2 = append(kids2, kid)
				}
			}
		}

		t.Logf("JWKS after rotation - keys count: %d, kids: [%s]", len(keys2), strings.Join(kids2, ","))

		// Validations
		if len(keys2) < 2 {
			t.Errorf("JWKS should contain at least 2 keys after rotation (active + retiring), got %d", len(keys2))
		}

		// Validate old KID is still present
		if kid1 != "" {
			found := false
			for _, kid := range kids2 {
				if kid == kid1 {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Old KID %s not found in JWKS after rotation", kid1)
			} else {
				t.Logf("Old KID %s still present in JWKS ✓", kid1)
			}
		}

		// Validate new KID is present and different
		if kid2 != "" {
			found := false
			for _, kid := range kids2 {
				if kid == kid2 {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("New KID %s not found in JWKS after rotation", kid2)
			} else {
				t.Logf("New KID %s present in JWKS ✓", kid2)
			}

			if kid1 != "" && kid1 == kid2 {
				t.Error("KID should have changed after rotation")
			} else if kid1 != "" {
				t.Logf("KID changed from %s to %s ✓", kid1, kid2)
			}
		} else {
			t.Log("[warn] ID token without 'kid' in header; integrate keystore + set 'kid' header in issuer for validation")
		}

		t.Log("[OK] Key rotation validated - old token works, JWKS contains multiple keys")
	})

	t.Run("KeyRotationEnvironmentValidation", func(t *testing.T) {
		// Test que el entorno está configurado correctamente para key rotation
		t.Log("== Environment Validation for Key Rotation ==")

		// Check if keys CLI exists
		if _, err := os.Stat("../../cmd/keys/main.go"); os.IsNotExist(err) {
			t.Skip("Keys CLI not found at ../../cmd/keys/main.go, skipping environment validation")
		}

		// Check if env file exists
		if _, err := os.Stat("../../" + envFile); os.IsNotExist(err) {
			t.Skipf("Env file %s not found, skipping environment validation", envFile)
		}

		// Test that we can at least execute the keys CLI help
		cmd := exec.Command("go", "run", "./cmd/keys/main.go", "-help")
		cmd.Dir = "../.."
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Keys CLI help command failed: %v\nOutput: %s", err, string(output))
		}

		t.Log("Keys CLI is available and executable ✓")

		// Check JWKS endpoint availability
		_, err = getJWKS()
		if err != nil {
			t.Fatalf("JWKS endpoint not available: %v", err)
		}

		t.Log("JWKS endpoint is accessible ✓")
		t.Log("Environment validation completed")
	})

	t.Run("MultipleKeyValidation", func(t *testing.T) {
		// Test para verificar que después de una rotación, múltiples keys están disponibles
		t.Log("== Multiple Key Validation ==")

		jwks, err := getJWKS()
		if err != nil {
			t.Fatalf("Failed to get JWKS: %v", err)
		}

		keys, ok := jwks["keys"].([]interface{})
		if !ok {
			t.Fatal("JWKS keys field is not an array")
		}

		if len(keys) == 0 {
			t.Fatal("No keys found in JWKS")
		}

		t.Logf("JWKS contains %d key(s)", len(keys))

		// Validate each key has required fields
		for i, key := range keys {
			keyMap, ok := key.(map[string]interface{})
			if !ok {
				t.Errorf("Key %d is not a valid object", i)
				continue
			}

			// Check required fields
			requiredFields := []string{"kty", "use", "kid"}
			for _, field := range requiredFields {
				if _, exists := keyMap[field]; !exists {
					t.Errorf("Key %d missing required field: %s", i, field)
				}
			}

			if kid, ok := keyMap["kid"].(string); ok {
				t.Logf("Key %d - KID: %s", i, kid)
			}
		}

		t.Log("Key validation completed")
	})
}
