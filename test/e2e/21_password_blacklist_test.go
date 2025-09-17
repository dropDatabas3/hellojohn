package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// 21 - Password blacklist: attempt register with a known weak password and expect 400/422 style response
func Test_21_Password_Blacklist(t *testing.T) {
	c := newHTTPClient()

	weakPwd := "password123" // common weak password expected in blacklist
	email := uniqueEmail(seed.Users.Admin.Email, "blacklist")

	body, _ := json.Marshal(map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     email,
		"password":  weakPwd,
	})
	resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		b, _ := io.ReadAll(resp.Body)
		// Falla duro: el password débil no debería ser aceptado
		if len(b) > 200 { b = b[:200] }
		 t.Fatalf("expected rejection for weak password, got %d body=%s", resp.StatusCode, string(b))
	}

	// Acceptable rejection codes: 400 (invalid), 422 (validation), 409 (conflict) if reused; we assert not success and mention policy if body includes it
	if resp.StatusCode >= 500 {
		b, _ := io.ReadAll(resp.Body)
		if len(b) > 200 { b = b[:200] }
		 t.Fatalf("server error unexpected for blacklist test: %d body=%s", resp.StatusCode, string(b))
	}
}
