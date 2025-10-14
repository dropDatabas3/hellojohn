package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 42 - HA: RequireLeader wiring canary
// Objetivo: detección temprana (fast smoke) de una regresión en el gating de rutas que mutan el plano de control FS.
// Rutas cubiertas (mutadoras actuales):
//   - POST /v1/admin/tenants            (creación/upsert tenant)
//   - PUT  /v1/admin/tenants/{slug}/scopes
//   - POST /v1/admin/tenants/{slug}/keys/rotate
//
// Expectativa: ejecutadas contra un follower deben devolver 409 + header X-Leader (y NO 200/201).
// Líder: la misma operación no debe devolver 409 (puede ser 401/403/200 según auth). Usamos token falso para forzar paso por chain sin depender de login.
// Extensión futura: si se agregan nuevas rutas mutadoras del FS (settings, clients upsert granular, etc.) añadirlas aquí para mantener cobertura canaria.
// Tiempo objetivo: < 3s en local (no crítico si >3s pero se loggea warning).
func Test_42_RequireLeader_Wiring_Smoke(t *testing.T) {
	if os.Getenv("STORAGE_DRIVER") == "" || os.Getenv("STORAGE_DSN") == "" {
		t.Skip("requires DB envs")
	}
	start := time.Now()
	root := t.TempDir()
	fs1 := filepath.Join(root, "n1")
	fs2 := filepath.Join(root, "n2")
	fs3 := filepath.Join(root, "n3")
	for _, d := range []string{fs1, fs2, fs3} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	peers := map[string]string{"node1": "127.0.0.1:18201", "node2": "127.0.0.1:18202", "node3": "127.0.0.1:18203"}
	redirects := map[string]string{"node1": "http://127.0.0.1:18081", "node2": "http://127.0.0.1:18082", "node3": "http://127.0.0.1:18083"}
	dsn := os.Getenv("STORAGE_DSN")
	for _, p := range []string{fs1, fs2, fs3} {
		if err := seedTenantFS(p, seed.Tenant.ID, "local", "Local", dsn); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = os.Setenv("POSTGRES_MAX_OPEN_CONNS", "1")
	_ = os.Setenv("POSTGRES_MAX_IDLE_CONNS", "1")
	_ = os.Setenv("CLUSTER_BOOTSTRAP", "1")
	n1, b1, err := startClusterNode(ctx, 18081, 18201, fs1, "node1", peers, redirects)
	if err != nil {
		t.Fatalf("node1: %v", err)
	}
	t.Cleanup(func() {
		if n1 != nil {
			n1.stop()
		}
	})
	_ = os.Unsetenv("CLUSTER_BOOTSTRAP")
	n2, b2, err := startClusterNode(ctx, 18082, 18202, fs2, "node2", peers, redirects)
	if err != nil {
		t.Fatalf("node2: %v", err)
	}
	t.Cleanup(func() {
		if n2 != nil {
			n2.stop()
		}
	})
	n3, b3, err := startClusterNode(ctx, 18083, 18203, fs3, "node3", peers, redirects)
	if err != nil {
		t.Fatalf("node3: %v", err)
	}
	t.Cleanup(func() {
		if n3 != nil {
			n3.stop()
		}
	})

	for _, b := range []string{b1, b2, b3} {
		if err := waitReady(b, 30*time.Second); err != nil {
			t.Fatalf("ready %s: %v", b, err)
		}
	}

	// Identify leader and a follower quickly
	type rz struct {
		Cluster struct {
			Role     string `json:"role"`
			LeaderID string `json:"leader_id"`
		} `json:"cluster"`
	}
	fetch := func(base string) (rz, error) {
		var out rz
		resp, err := http.Get(base + "/readyz")
		if err != nil {
			return out, err
		}
		defer resp.Body.Close()
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out, nil
	}
	bases := []string{b1, b2, b3}
	var leaderBase, followerBase string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, b := range bases {
			st, _ := fetch(b)
			if st.Cluster.Role == "leader" {
				leaderBase = b
			}
		}
		if leaderBase != "" {
			for _, b := range bases {
				if b == leaderBase {
					continue
				}
				st, _ := fetch(b)
				if st.Cluster.Role == "follower" && st.Cluster.LeaderID != "" {
					followerBase = b
					break
				}
			}
		}
		if leaderBase != "" && followerBase != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if leaderBase == "" || followerBase == "" {
		t.Skip("no leader/follower in time")
	}

	// Test a minimal set of gated routes against follower expecting 409
	gated := []struct{ method, path string }{
		{"POST", "/v1/admin/tenants"},
		{"PUT", "/v1/admin/tenants/local/scopes"},
		{"POST", "/v1/admin/tenants/local/keys/rotate"},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	for _, r := range gated {
		req, _ := http.NewRequest(r.method, followerBase+r.path, nil)
		// We intentionally do NOT set Authorization; depending on chain order we may see 401 first.
		// If 401 occurs consistently the canary loses value; try with a fake token to force deeper middleware.
		req.Header.Set("Authorization", "Bearer fake-token")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, r.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusConflict { // accept 409 only
			t.Fatalf("expected 409 on follower for %s %s got %d", r.method, r.path, resp.StatusCode)
		}
		if l := resp.Header.Get("X-Leader"); l == "" {
			t.Fatalf("missing X-Leader header on %s %s", r.method, r.path)
		}
	}

	// Quick sanity: same call on leader should not return 409 (expect 401/403/200 depending on auth)
	req, _ := http.NewRequest("POST", leaderBase+"/v1/admin/tenants", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusConflict {
			t.Fatalf("leader returned 409 unexpected (status=%d)", resp.StatusCode)
		}
	}

	dur := time.Since(start)
	if dur > 3*time.Second {
		t.Logf("WARNING: canary duration %s > 3s target", dur)
	}
	t.Logf("Canary wiring OK in %s (leader=%s follower=%s)", dur, leaderBase, followerBase)
}
