package e2e

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

func Test_Introspect_BasicAuth(t *testing.T) {
	user := os.Getenv("INTROSPECT_BASIC_USER")
	pass := os.Getenv("INTROSPECT_BASIC_PASS")
	if user == "" || pass == "" {
		t.Skip("INTROSPECT_BASIC_USER/PASS no configurados; skipping")
	}

	c := newHTTPClient()

	// 1) login para conseguir un access_token
	type loginReq struct {
		TenantID string `json:"tenant_id"`
		ClientID string `json:"client_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type loginResp struct {
		AccessToken string `json:"access_token"`
	}
	lr := loginReq{
		TenantID: seed.Tenant.ID,
		ClientID: seed.Clients.Web.ClientID,
		Email:    seed.Users.Admin.Email,
		Password: seed.Users.Admin.Password,
	}
	body, _ := jsonMarshal(lr)
	resp, err := c.Post(baseURL+"/v1/auth/login", "application/json", strings.NewReader(string(body)))
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
	if lout.AccessToken == "" {
		t.Fatalf("access token vacío")
	}

	// 2) introspect válido (active=true)
	form := url.Values{}
	form.Set("token", lout.AccessToken)
	req, _ := http.NewRequest("POST", baseURL+"/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))

	r2, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != 200 {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("introspect status=%d body=%s", r2.StatusCode, string(b))
	}
	var iout struct {
		Active bool `json:"active"`
	}
	if err := mustJSON(r2.Body, &iout); err != nil {
		t.Fatal(err)
	}
	if !iout.Active {
		t.Fatalf("se esperaba active=true para un access_token válido")
	}

	// 3) introspect inválido (active=false)
	form = url.Values{}
	form.Set("token", "not-a-real-token")
	reqBad, _ := http.NewRequest("POST", baseURL+"/oauth2/introspect", strings.NewReader(form.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBad.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	r3, err := c.Do(reqBad)
	if err != nil {
		t.Fatal(err)
	}
	defer r3.Body.Close()
	if r3.StatusCode != 200 {
		t.Fatalf("introspect (token inválido) status=%d", r3.StatusCode)
	}
	var iout2 struct {
		Active bool `json:"active"`
	}
	if err := mustJSON(r3.Body, &iout2); err != nil {
		t.Fatal(err)
	}
	if iout2.Active {
		t.Fatalf("se esperaba active=false para token inválido")
	}
}
