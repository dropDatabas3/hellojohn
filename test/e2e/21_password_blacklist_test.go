package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 21 - Password blacklist: attempt register with a known weak password and expect 400/422 style response
func Test_21_Password_Blacklist(t *testing.T) {
	c := newHTTPClient()

	blacklistPath := os.Getenv("SECURITY_PASSWORD_BLACKLIST_PATH")
	if blacklistPath == "" {
		t.Skip("password blacklist disabled (env not set)")
	}
	// Resolver absoluto usando repo root (buscando go.mod hacia arriba desde cwd)
	cwd, _ := os.Getwd()
	root := cwd
	for i := 0; i < 6; i++ { // subir hasta 6 niveles por seguridad
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root { // llegó al FS root
			break
		}
		root = parent
	}
	if !filepath.IsAbs(blacklistPath) {
		blacklistPath = filepath.Clean(filepath.Join(root, blacklistPath))
	}
	if st, err := os.Stat(blacklistPath); err != nil || st.IsDir() {
		t.Skip("password blacklist file not found; skipping")
	}

	weakPwd := "password123" // fallback
	if f, err := os.Open(blacklistPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			ln := strings.TrimSpace(scanner.Text())
			if ln == "" || strings.HasPrefix(ln, "#") {
				continue
			}
			weakPwd = ln
			break
		}
		_ = f.Close()
	}

	email := uniqueEmail(seed.Users.Admin.Email, "blacklist")

	body, _ := json.Marshal(map[string]any{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     email,
		"password":  weakPwd,
	})
	resp, err := c.Post(baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		b, _ := io.ReadAll(resp.Body)
		if len(b) > 200 {
			b = b[:200]
		}
		pathHdr := resp.Header.Get("X-Debug-Blacklist-Path")
		hitHdr := resp.Header.Get("X-Debug-Blacklist-Hit")
		errHdr := resp.Header.Get("X-Debug-Blacklist-Err")
		t.Fatalf("expected rejection for weak password, got %d body=%s debug_path=%s hit=%s err=%s", resp.StatusCode, string(b), pathHdr, hitHdr, errHdr)
	}

	// Acceptable rejection codes: 400 (invalid), 422 (validation), 409 (conflict) if reused; we assert not success and mention policy if body includes it
	if resp.StatusCode >= 500 {
		b, _ := io.ReadAll(resp.Body)
		if len(b) > 200 {
			b = b[:200]
		}
		t.Fatalf("server error unexpected for blacklist test: %d body=%s", resp.StatusCode, string(b))
	}
}
