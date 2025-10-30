package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 46 - Leader redirect whitelist: 307 when allowed, 409 when not allowed
func Test_46_Leader_Redirect_Allowlist(t *testing.T) {
	if os.Getenv("STORAGE_DRIVER") == "" || os.Getenv("STORAGE_DSN") == "" {
		t.Skip("requires DB to login admin for admin write")
	}
	root := t.TempDir()
	fs1 := filepath.Join(root, "wl1")
	fs2 := filepath.Join(root, "wl2")
	for _, d := range []string{fs1, fs2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	peers := map[string]string{
		"wl1": "127.0.0.1:19401",
		"wl2": "127.0.0.1:19402",
	}
	redirects := map[string]string{
		"wl1": "http://127.0.0.1:19381",
		"wl2": "http://127.0.0.1:19382",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Start leader with bootstrap
	_ = os.Setenv("CLUSTER_BOOTSTRAP", "1")
	n1, b1, err := startClusterNode(ctx, 19381, 19401, fs1, "wl1", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if n1 != nil {
			n1.stop()
		}
	})
	_ = os.Unsetenv("CLUSTER_BOOTSTRAP")
	// Follower with allowlist including leader host
	n2, b2, err := startClusterNodeWithEnv(ctx, 19382, 19402, fs2, "wl2", peers, redirects, map[string]string{
		"LEADER_REDIRECT_ALLOWED_HOSTS": "127.0.0.1:19381",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if n2 != nil {
			n2.stop()
		}
	})
	for _, base := range []string{b1, b2} {
		if err := waitReady(base, 35*time.Second); err != nil {
			t.Fatal(err)
		}
	}

	// login admin via follower
	loginBody := map[string]string{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	}
	buf, _ := json.Marshal(loginBody)
	resp, err := newHTTPClient().Post(b2+"/v1/auth/login", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("login failed: %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	if tok.AccessToken == "" {
		t.Skip("no token")
	}

	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	post := func(base string) (int, string) {
		slug := "wl-" + itoa(time.Now().Unix()%1e6) + "-" + itoa(int64(rand.Intn(1000)))
		doc := map[string]any{"slug": slug, "name": "WL"}
		data, _ := json.Marshal(doc)
		req, _ := http.NewRequest(http.MethodPost, base+"/v1/admin/tenants?leader_redirect=1", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("X-Leader-Redirect", "1")
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)
		if resp == nil {
			return 0, ""
		}
		loc := readHeader(resp, "Location")
		_ = resp.Body.Close()
		return resp.StatusCode, loc
	}
	// Expect 307 because allowlist permits leader host
	code, loc := post(b2)
	if code != http.StatusTemporaryRedirect || loc == "" {
		t.Fatalf("expected 307 with Location, got %d loc=%q", code, loc)
	}

	// Restart follower with allowlist excluding leader -> expect 409
	n2.stop()
	n2, _, err = startClusterNodeWithEnv(ctx, 19382, 19402, fs2, "wl2", peers, redirects, map[string]string{
		"LEADER_REDIRECT_ALLOWED_HOSTS": "127.0.0.1:12345", // wrong
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if n2 != nil {
			n2.stop()
		}
	}()
	if err := waitReady(b2, 25*time.Second); err != nil {
		t.Fatal(err)
	}
	code, loc = post(b2)
	if code != http.StatusConflict || loc != "" {
		t.Fatalf("expected 409 without Location, got %d loc=%q", code, loc)
	}
}
