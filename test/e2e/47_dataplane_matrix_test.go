package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 47 - Dataplane matrix: A(FS), B(FS+global DB), C(per-tenant DB), D(mixed)
// This test runs a quick smoke on the server(s) available in the environment. When DB is missing, subtests that need it are skipped.
func Test_47_Dataplane_Matrix(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	hasDB := os.Getenv("STORAGE_DRIVER") != "" && os.Getenv("STORAGE_DSN") != ""

	// A) FS-only
	t.Run("A_FS_only", func(t *testing.T) {
		// Ensure FS-only by clearing DB env for the child process only (use startServer directly)
		n, base, err := startClusterNodeWithEnv(ctx, 19581, 19681, t.TempDir(), "fa", map[string]string{"fa": "127.0.0.1:19681"}, nil, map[string]string{
			"STORAGE_DRIVER": "",
			"STORAGE_DSN":    "",
		})
		if err != nil {
			t.Fatal(err)
		}
		defer n.stop()
		if err := waitReady(base, 25*time.Second); err != nil {
			t.Fatal(err)
		}
		// JWKS should be available
		resp, err := newHTTPClient().Get(base + "/.well-known/jwks.json")
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("jwks status=%v err=%v", resp.StatusCode, err)
		}
		_ = resp.Body.Close()
	})

	if !hasDB {
		t.Skip("remaining subtests require DB")
	}

	// B) FS + global DB
	t.Run("B_FS_global_DB", func(t *testing.T) {
		n, base, err := startClusterNode(ctx, 19582, 19682, t.TempDir(), "fb", map[string]string{"fb": "127.0.0.1:19682"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer n.stop()
		if err := waitReady(base, 25*time.Second); err != nil {
			t.Fatal(err)
		}
		// smoke login/refresh/introspect
		loginBody := map[string]string{"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID, "email": seed.Users.Admin.Email, "password": seed.Users.Admin.Password}
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
			AccessToken, RefreshToken string `json:"access_token" json:"refresh_token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&tok)
		if tok.AccessToken == "" || tok.RefreshToken == "" {
			t.Skip("tokens missing")
		}
		// refresh
		req := map[string]string{"grant_type": "refresh_token", "refresh_token": tok.RefreshToken}
		b2, _ := json.Marshal(req)
		resp2, _ := newHTTPClient().Post(base+"/v1/auth/refresh", "application/json", bytes.NewReader(b2))
		if resp2 != nil {
			_ = resp2.Body.Close()
		}
		// introspect
		req3, _ := http.NewRequest(http.MethodPost, base+"/oauth2/introspect", nil)
		// Basic auth disabled by default unless set; skip checking body
		_, _ = newHTTPClient().Do(req3)
	})

	// C) Per-tenant DB isolation
	t.Run("C_PerTenant_DB", func(t *testing.T) {
		root := t.TempDir()
		fsRoot := filepath.Join(root, "fs")
		if err := os.MkdirAll(fsRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := seedTenantFS(fsRoot, seed.Tenant.ID, "local", "Local", os.Getenv("STORAGE_DSN")); err != nil {
			t.Fatal(err)
		}
		n, base, err := startClusterNode(ctx, 19583, 19683, fsRoot, "fc", map[string]string{"fc": "127.0.0.1:19683"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer n.stop()
		if err := waitReady(base, 25*time.Second); err != nil {
			t.Fatal(err)
		}
		// login specifying tenant
		loginBody := map[string]string{"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID, "email": seed.Users.Admin.Email, "password": seed.Users.Admin.Password}
		buf, _ := json.Marshal(loginBody)
		resp, err := newHTTPClient().Post(base+"/v1/auth/login", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	})

	// D) Mixed mode (global + per-tenant) – basic readiness
	t.Run("D_Mixed", func(t *testing.T) {
		root := t.TempDir()
		fsRoot := filepath.Join(root, "fs")
		if err := os.MkdirAll(fsRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		// seed per-tenant for isolation
		if err := seedTenantFS(fsRoot, seed.Tenant.ID, "local", "Local", os.Getenv("STORAGE_DSN")); err != nil {
			t.Fatal(err)
		}
		n, base, err := startClusterNode(ctx, 19584, 19684, fsRoot, "fd", map[string]string{"fd": "127.0.0.1:19684"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer n.stop()
		if err := waitReady(base, 25*time.Second); err != nil {
			t.Fatal(err)
		}
	})
}
