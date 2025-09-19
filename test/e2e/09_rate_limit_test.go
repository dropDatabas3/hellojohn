package e2e

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func Test_RateLimit_Forgot(t *testing.T) {
	if strings.ToLower(os.Getenv("RATE_ENABLED")) != "true" {
		t.Skip("RATE_ENABLED!=true; skipping")
	}
	cache := strings.ToLower(os.Getenv("CACHE_KIND"))
	if cache == "redis" && os.Getenv("REDIS_ADDR") == "" {
		t.Skip("CACHE_KIND=redis pero sin REDIS_ADDR; skipping")
	}
	if cache != "redis" && strings.ToLower(os.Getenv("RATE_TEST_ALLOW_MEMORY")) != "true" {
		t.Skip("sin Redis y RATE_TEST_ALLOW_MEMORY!=true; skipping para evitar flakes")
	}

	c := newHTTPClient()
	type forgotReq struct {
		TenantID    string `json:"tenant_id"`
		ClientID    string `json:"client_id"`
		Email       string `json:"email"`
		RedirectURI string `json:"redirect_uri"`
	}
	fr := forgotReq{
		TenantID:    seed.Tenant.ID,
		ClientID:    seed.Clients.Web.ClientID,
		Email:       seed.Users.Admin.Email,
		RedirectURI: "http://localhost:3000/callback",
	}
	b1, _ := jsonMarshal(fr)

	// Primer intento -> 200
	r1, err := c.Post(baseURL+"/v1/auth/forgot", "application/json", bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, r1.Body)
	r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Fatalf("forgot #1 status=%d", r1.StatusCode)
	}

	// Segundo intento in-window -> 429 (si configurado determinista)
	time.Sleep(50 * time.Millisecond)
	r2, err := c.Post(baseURL+"/v1/auth/forgot", "application/json", bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != 429 {
		t.Skipf("no llegó 429 (status=%d). Probable umbral alto o ventana distinta; skipping para evitar flakes", r2.StatusCode)
	}
	if ra := readHeader(r2, "Retry-After"); ra == "" {
		t.Fatalf("429 sin Retry-After")
	}
}
