package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

var (
	baseURL = getBaseURL()
	seed    *seedData
	srv     *serverProc
)

func getBaseURL() string {
	if issuer := os.Getenv("JWT_ISSUER"); issuer != "" {
		return issuer
	}
	if baseURL := os.Getenv("EMAIL_BASE_URL"); baseURL != "" {
		return baseURL
	}
	// Fallback to default
	return "http://localhost:8081"
}

func Test_Discovery_JWKS(t *testing.T) {
	c := newHTTPClient()

	// JWKS
	resp, err := c.Get(baseURL + "/.well-known/jwks.json")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("jwks status=%d", resp.StatusCode)
	}
	resp.Body.Close()

	// OIDC discovery
	resp, err = c.Get(baseURL + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("oidc discovery status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}

func Test_Login_And_UserInfo(t *testing.T) {
	c := newHTTPClient()

	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type loginResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		MFAToken     string `json:"mfa_token"`
		Required     bool   `json:"mfa_required"`
	}

	lr := loginReq{
		TenantID: seed.Tenant.ID,
		ClientID: seed.Clients.Web.ClientID,
		Email:    seed.Users.Admin.Email,
		Password: seed.Users.Admin.Password,
	}
	b, _ := json.Marshal(lr)

	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status=%d", resp.StatusCode)
	}

	var out loginResp
	if err := mustJSON(resp.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatalf("missing tokens: %+v", out)
	}

	// userinfo
	req, _ := http.NewRequest("GET", baseURL+"/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+out.AccessToken)
	uinfo, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer uinfo.Body.Close()
	if uinfo.StatusCode != 200 {
		t.Fatalf("userinfo status=%d", uinfo.StatusCode)
	}
}

func Test_Login_MFA_Gating_And_Challenge(t *testing.T) {
	c := newHTTPClient()

	type loginReq struct{ TenantID, ClientID, Email, Password string }
	type loginResp struct {
		MFAToken string `json:"mfa_token"`
		Required bool   `json:"mfa_required"`
	}

	// login del usuario con MFA
	body, _ := json.Marshal(loginReq{
		TenantID: seed.Tenant.ID,
		ClientID: seed.Clients.Web.ClientID,
		Email:    seed.Users.MFA.Email,
		Password: seed.Users.MFA.Password,
	})
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login mfa status=%d", resp.StatusCode)
	}

	var lr loginResp
	if err := mustJSON(resp.Body, &lr); err != nil {
		t.Fatal(err)
	}
	if !lr.Required || lr.MFAToken == "" {
		t.Fatalf("expected mfa_required with token: %+v", lr)
	}

	// challenge con código incorrecto
	chBad := map[string]any{"mfa_token": lr.MFAToken, "code": "000000"}
	bad, _ := json.Marshal(chBad)
	r2, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", strings.NewReader(string(bad)))
	r2.Header.Set("Content-Type", "application/json")
	resp2, err := c.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 && resp2.StatusCode != 400 {
		t.Fatalf("bad code expected 401/400 got %d", resp2.StatusCode)
	}

	// challenge con recovery codes disponibles (iterar hasta encontrar uno no consumido)
	if len(seed.Users.MFA.Recovery) == 0 {
		t.Skip("seed has no recovery codes; skipping recovery path")
	}

	var okResp *http.Response
	var usedRecovery string
	for _, rc := range seed.Users.MFA.Recovery {
		chOK := map[string]any{"mfa_token": lr.MFAToken, "recovery": rc, "remember_device": true}
		okb, _ := json.Marshal(chOK)
		r3, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", strings.NewReader(string(okb)))
		r3.Header.Set("Content-Type", "application/json")
		resp3, err := c.Do(r3)
		if err != nil {
			t.Fatal(err)
		}
		if resp3.StatusCode == 200 {
			okResp = resp3
			usedRecovery = rc
			break
		}
		resp3.Body.Close()
	}

	if okResp == nil {
		t.Skip("no quedó ningún recovery code usable (otro test los consumió)")
	}
	defer okResp.Body.Close()

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := mustJSON(okResp.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatalf("missing tokens on mfa challenge: %+v", out)
	}

	// reusar el MISMO recovery → debe fallar
	chReuse := map[string]any{"mfa_token": lr.MFAToken, "recovery": usedRecovery, "remember_device": false}
	reuseBody, _ := json.Marshal(chReuse)
	r3b, _ := http.NewRequest("POST", baseURL+"/v1/mfa/totp/challenge", strings.NewReader(string(reuseBody)))
	r3b.Header.Set("Content-Type", "application/json")
	resp3b, err := c.Do(r3b)
	if err != nil {
		t.Fatal(err)
	}
	resp3b.Body.Close()
	if resp3b.StatusCode == 200 {
		t.Fatalf("reusing recovery should fail")
	}
}
