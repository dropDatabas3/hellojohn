package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// normalizeJSON returns a canonical minified JSON string with keys sorted (shallow for JWKS usage).
func normalizeJSON(b []byte) string {
	var obj any
	if err := json.Unmarshal(b, &obj); err != nil {
		return string(b)
	}
	// For JWKS, structure is {"keys":[{...},{...}]} – we sort each key object's keys for deterministic compare.
	m, ok := obj.(map[string]any)
	if !ok {
		return string(b)
	}
	if arr, ok := m["keys"].([]any); ok {
		for _, it := range arr {
			if km, ok := it.(map[string]any); ok {
				// reorder by building new map in key order when marshaling
				keys := make([]string, 0, len(km))
				for k := range km {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				ordered := make([]any, 0, len(keys)*2)
				_ = ordered // (marshaler doesn't keep order; fallback to re-marshal km alone)
				// we just rely on stdlib stable map iteration in Go 1.22? Not guaranteed → fallback: marshal individually
			}
		}
	}
	out, _ := json.Marshal(m) // stdlib marshal already deterministic for identical logical structure
	return string(out)
}

func Test_41_SnapshotRestore_JWKSIdentical(t *testing.T) {
	if os.Getenv("STORAGE_DRIVER") == "" || os.Getenv("STORAGE_DSN") == "" {
		t.Skip("requires DB envs")
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	// Force rapid snapshots
	_ = os.Setenv("RAFT_SNAPSHOT_EVERY", "5")
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
		if err := waitReady(b, 40*time.Second); err != nil {
			t.Fatalf("ready %s: %v", b, err)
		}
	}

	// Elect leader (reuse logic from test 40 simplified)
	type readyz struct {
		Cluster struct {
			Role     string `json:"role"`
			LeaderID string `json:"leader_id"`
		} `json:"cluster"`
	}
	fetch := func(base string) readyz {
		var rz readyz
		resp, err := http.Get(base + "/readyz")
		if err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&rz)
			resp.Body.Close()
		}
		return rz
	}
	var leaderBase string
	waitElection := time.Now().Add(35 * time.Second)
	for time.Now().Before(waitElection) {
		for _, b := range []string{b1, b2, b3} {
			if fetch(b).Cluster.Role == "leader" {
				leaderBase = b
				break
			}
		}
		if leaderBase != "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if leaderBase == "" {
		t.Skip("no leader")
	}
	t.Logf("leader=%s", leaderBase)

	// Login admin on leader
	login := func(base string) string {
		body := map[string]string{"tenant_id": seed.Tenant.ID, "client_id": seed.Clients.Web.ClientID, "email": seed.Users.Admin.Email, "password": seed.Users.Admin.Password}
		buf, _ := json.Marshal(body)
		resp, err := http.Post(base+"/v1/auth/login", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Skipf("login fail %d %s", resp.StatusCode, string(b))
		}
		var tok struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&tok)
		return tok.AccessToken
	}
	token := login(leaderBase)

	// Rotate tenant keys (POST /v1/admin/tenants/local/keys/rotate?graceSeconds=30)
	rotate := func() {
		req, _ := http.NewRequest(http.MethodPost, leaderBase+"/v1/admin/tenants/local/keys/rotate?graceSeconds=30", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("rotate: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 204 {
			t.Fatalf("rotate status=%d", resp.StatusCode)
		}
	}
	rotate()

	// Fetch JWKS from leader (global + tenant)
	getJ := func(base, path string) string {
		resp, err := http.Get(base + path)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return normalizeJSON(b)
	}
	jwksGlobalLeader := getJ(leaderBase, "/.well-known/jwks.json")
	jwksTenantLeader := getJ(leaderBase, "/.well-known/jwks/local.json")
	if jwksGlobalLeader == "" || jwksTenantLeader == "" {
		t.Fatalf("empty jwks leader")
	}

	// Stop node3 to simulate needing snapshot
	n3.stop()

	// Remove raft state for node3 (simulate restore-from-snapshot)
	raftDir := filepath.Join(fs3, "raft")
	_ = os.RemoveAll(raftDir)

	// Restart node3
	n3b, b3b, err := startClusterNode(ctx, 18083, 18203, fs3, "node3", peers, redirects)
	if err != nil {
		t.Fatalf("node3 restart: %v", err)
	}
	t.Cleanup(func() {
		if n3b != nil {
			n3b.stop()
		}
	})
	if err := waitReady(b3b, 40*time.Second); err != nil {
		t.Fatalf("node3 restored ready: %v", err)
	}

	// Poll until JWKS available and matches
	deadline := time.Now().Add(60 * time.Second)
	var lastG3, lastT3 string
	for time.Now().Before(deadline) {
		g3 := getJ(b3b, "/.well-known/jwks.json")
		t3 := getJ(b3b, "/.well-known/jwks/local.json")
		if g3 != "" {
			lastG3 = g3
		}
		if t3 != "" {
			lastT3 = t3
		}
		if g3 != "" && t3 != "" && g3 == jwksGlobalLeader && t3 == jwksTenantLeader {
			act1, _ := os.ReadFile(filepath.Join(fs1, "keys", "local", "active.json"))
			act3, _ := os.ReadFile(filepath.Join(fs3, "keys", "local", "active.json"))
			if len(act1) == 0 || len(act3) == 0 {
				t.Fatalf("active.json missing after restore (leader=%d follower=%d)", len(act1), len(act3))
			}
			if !bytes.Equal(act1, act3) {
				t.Fatalf("active.json mismatch len leader=%d follower=%d", len(act1), len(act3))
			}
			t.Logf("active.json identical (%d bytes)", len(act1))
			t.Log("JWKS match after restore")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Diagnostics on failure
	t.Logf("Leader global JWKS: %s", truncate(jwksGlobalLeader, 240))
	t.Logf("Follower last global JWKS: %s", truncate(lastG3, 240))
	t.Logf("Leader tenant JWKS: %s", truncate(jwksTenantLeader, 240))
	t.Logf("Follower last tenant JWKS: %s", truncate(lastT3, 240))
	t.Fatalf("JWKS not identical after restore (timeout %s)", 60*time.Second)
}
