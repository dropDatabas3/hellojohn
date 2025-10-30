package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// 45 - IO fault injection: force FS write errors and assert readyz fs_degraded=true
func Test_45_FS_Degraded_On_Write_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based fault injection not reliable on Windows")
	}
	if os.Getenv("STORAGE_DRIVER") == "" || os.Getenv("STORAGE_DSN") == "" {
		t.Skip("requires DB to login admin for admin write")
	}
	root := t.TempDir()
	fsRoot := filepath.Join(root, "fs")
	if err := os.MkdirAll(filepath.Join(fsRoot, "tenants", "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	n, base, err := startClusterNode(ctx, 19101, 19301, fsRoot, "fsbad1", map[string]string{"fsbad1": "127.0.0.1:19301"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if n != nil {
			n.stop()
		}
	})
	if err := waitReady(base, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	// Make tenants directory read-only to cause atomic write failures
	tenDir := filepath.Join(fsRoot, "tenants", "fault")
	if err := os.MkdirAll(tenDir, 0o555); err != nil {
		t.Fatal(err)
	}
	// login admin
	loginBody := map[string]string{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	}
	buf, _ := json.Marshal(loginBody)
	resp, err := newHTTPClient().Post(base+"/v1/auth/login", "application/json", bytes.NewReader(buf))
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

	// Attempt to upsert tenant under the read-only dir to force error
	doc := map[string]any{"slug": "fault", "name": "Fault"}
	data, _ := json.Marshal(doc)
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/admin/tenants", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	_, _ = newHTTPClient().Do(req)
	// Now read readyz and assert fs_degraded
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp2, err := newHTTPClient().Get(base + "/readyz")
		if err == nil {
			var v struct {
				FSDegraded bool `json:"fs_degraded"`
			}
			_ = json.NewDecoder(resp2.Body).Decode(&v)
			_ = resp2.Body.Close()
			if v.FSDegraded {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("fs_degraded did not turn true after write failure")
}
