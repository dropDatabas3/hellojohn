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

// 26 - Token & ID Token Claims: amr en IDT; scp opcional en AT (array)
func Test_26_Token_And_IDToken_Claims(t *testing.T) {
	c := newHTTPClient()

	// 1) Session login (cookie) para flujo Auth Code + PKCE
	{
		body, _ := json.Marshal(map[string]any{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		})
		resp, err := c.Post(baseURL+"/v1/session/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			t.Fatalf("session login status=%d", resp.StatusCode)
		}
	}

	// 2) /oauth2/authorize (Code+PKCE)
	redirect := "http://localhost:3000/callback"
	if len(seed.Clients.Web.Redirects) > 0 && seed.Clients.Web.Redirects[0] != "" {
		redirect = seed.Clients.Web.Redirects[0]
	}

	noFollow := newHTTPClient()
	noFollow.Jar = c.Jar
	noFollow.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }

	verifier := newCodeVerifier()
	challenge := pkceS256(verifier)
	state := "st" + itoa(time.Now().Unix()%1_000_000)
	nonce := "nn" + itoa(time.Now().Unix()%1_000_000)

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", seed.Clients.Web.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	rAuth, err := noFollow.Get(baseURL + "/oauth2/authorize?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer rAuth.Body.Close()
	if rAuth.StatusCode != 302 {
		b, _ := io.ReadAll(rAuth.Body)
		t.Fatalf("authorize expected 302, got %d body=%s", rAuth.StatusCode, string(b))
	}
	loc := readHeader(rAuth, "Location")
	if !strings.Contains(loc, "code=") || !strings.Contains(loc, "state="+state) {
		t.Fatalf("authorize redirect missing code/state")
	}
	code := qs(loc, "code")

	// 3) /oauth2/token exchange
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("client_id", seed.Clients.Web.ClientID)
	form.Set("code_verifier", verifier)

	rTok, err := c.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer rTok.Body.Close()
	if rTok.StatusCode != 200 {
		b, _ := io.ReadAll(rTok.Body)
		t.Fatalf("token status=%d body=%s", rTok.StatusCode, string(b))
	}
	if cc := readHeader(rTok, "Cache-Control"); !strings.EqualFold(cc, "no-store") {
		t.Fatalf("token Cache-Control expected no-store got %q", cc)
	}
	if pg := readHeader(rTok, "Pragma"); !strings.EqualFold(pg, "no-cache") {
		t.Fatalf("token Pragma expected no-cache got %q", pg)
	}

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		Scope        string `json:"scope"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := mustJSON(rTok.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.IDToken == "" || out.AccessToken == "" {
		t.Fatalf("token response missing tokens: %+v", out)
	}

	// 4) Validar 'amr' en ID Token (debería incluir "pwd" tras login con password)
	hdrID, pldID, err := decodeJWT(out.IDToken)
	if err != nil {
		t.Fatalf("decode id_token: %v", err)
	}
	_ = hdrID
	amr, _ := pldID["amr"].([]any)
	if len(amr) == 0 {
		t.Errorf("id_token missing 'amr' claim (expected [\"pwd\"] or [\"pwd\",\"mfa\"])")
	} else {
		gotPwd := false
		for _, v := range amr {
			if s, _ := v.(string); strings.EqualFold(s, "pwd") {
				gotPwd = true
				break
			}
		}
		if !gotPwd {
			t.Errorf("id_token.amr does not include \"pwd\": %v", amr)
		}
	}

	// 5) Validar 'scp' opcional en Access Token (array equivalente a 'scope')
	hAT, pAT, err := decodeJWT(out.AccessToken)
	if err != nil {
		t.Fatalf("decode access_token: %v", err)
	}
	_ = hAT
	if x, ok := pAT["scp"]; ok {
		switch vv := x.(type) {
		case []any:
			// si viene, validar que coincida con los scopes de 'scope'
			want := strings.Fields(out.Scope)
			wantSet := map[string]bool{}
			for _, w := range want {
				wantSet[strings.ToLower(w)] = true
			}
			for _, e := range vv {
				if s, _ := e.(string); s != "" {
					if !wantSet[strings.ToLower(s)] {
						t.Errorf("scp contains unexpected scope %q (want subset of %v)", s, want)
					}
				}
			}
		default:
			t.Errorf("scp claim present but not an array: %T", vv)
		}
	} else {
		t.Log("access_token without 'scp' (allowed) – claim is optional for compat")
	}
}
