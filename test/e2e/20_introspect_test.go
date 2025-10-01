package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	jwti "github.com/dropDatabas3/hellojohn/test/e2e/internal"
)

// 20 - Introspection endpoint: valid access, valid refresh, revoked refresh, invalid token
func Test_20_Introspect(t *testing.T) {
	c := newHTTPClient()

	// 1. Login to obtain tokens
	// (local structs with correct JSON tags)
	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	lr := map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
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
	var tk tokenResp
	if err := mustJSON(resp.Body, &tk); err != nil {
		t.Fatal(err)
	}
	if tk.AccessToken == "" || tk.RefreshToken == "" {
		t.Fatalf("missing tokens: %+v", tk)
	}

	// Cryptographic verification of access token
	if _, _, err := jwti.VerifyJWTWithJWKS(context.Background(), baseURL, tk.AccessToken, baseURL, seed.Clients.Web.ClientID, 60*time.Second); err != nil {
		t.Fatalf("verify access with jwks: %v", err)
	}

	// Helper to introspect
	user := os.Getenv("INTROSPECT_BASIC_USER")
	pass := os.Getenv("INTROSPECT_BASIC_PASS")
	call := func(token string) (int, map[string]any) {
		reqBody := []byte("token=" + token)
		rq, _ := http.NewRequest("POST", baseURL+"/oauth2/introspect", bytes.NewReader(reqBody))
		rq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if user != "" || pass != "" {
			rq.SetBasicAuth(user, pass)
		}
		resp, err := c.Do(rq)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		body, _ := io.ReadAll(resp.Body)
		_ = json.Unmarshal(body, &out)
		return resp.StatusCode, out
	}

	// 2. Introspect access (should be active)
	st, out := call(tk.AccessToken)
	if st != 200 {
		t.Fatalf("introspect access status=%d", st)
	}
	if act, _ := out["active"].(bool); !act {
		t.Fatalf("expected active access")
	}

	// 3. Introspect refresh (active)
	st, out = call(tk.RefreshToken)
	if st != 200 {
		t.Fatalf("introspect refresh status=%d", st)
	}
	if act, _ := out["active"].(bool); !act {
		t.Fatalf("expected active refresh")
	}

	// 4. Revoke refresh via logout
	bodyLogout, _ := json.Marshal(map[string]any{"refresh_token": tk.RefreshToken})
	lrq, _ := http.NewRequest("POST", baseURL+"/v1/auth/logout", bytes.NewReader(bodyLogout))
	lrq.Header.Set("Content-Type", "application/json")
	lresp, err := c.Do(lrq)
	if err != nil {
		t.Fatal(err)
	}
	lresp.Body.Close()
	if lresp.StatusCode != 204 {
		t.Fatalf("logout status=%d", lresp.StatusCode)
	}

	// 5. Introspect revoked refresh -> inactive
	st, out = call(tk.RefreshToken)
	if st != 200 {
		t.Fatalf("introspect revoked status=%d", st)
	}
	if act, _ := out["active"].(bool); act {
		t.Fatalf("expected inactive after revoke")
	}

	// 6. Introspect garbage token
	st, out = call("ABC.NOT.A.JWT")
	if st != 200 {
		t.Fatalf("introspect invalid status=%d", st)
	}
	if act, _ := out["active"].(bool); act {
		t.Fatalf("garbage token should be inactive")
	}

	// Negative: mutate alg header locally -> verification must fail
	parts := strings.Split(tk.AccessToken, ".")
	if len(parts) >= 2 {
		var hdr map[string]any
		_ = json.Unmarshal(mustB64(parts[0]), &hdr)
		hdr["alg"] = "none"
		b, _ := json.Marshal(hdr)
		mut := b64url(b) + "." + parts[1] + "." + parts[2]
		if _, _, err := jwti.VerifyJWTWithJWKS(context.Background(), baseURL, mut, baseURL, seed.Clients.Web.ClientID, 60*time.Second); err == nil {
			t.Fatalf("expected failure on unexpected alg")
		}
	}
}
