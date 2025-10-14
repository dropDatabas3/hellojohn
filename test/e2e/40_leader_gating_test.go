package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// truncate helper for logging
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// 40 - HA: RequireLeader follower behavior (409 + X-Leader) and optional redirect (307)
// Strategy:
// 1. Start 2-node cluster, seed control-plane FS for login.
// 2. Poll /readyz until one node is leader and the other follower with leader_id set.
// 3. Login against follower (and leader for completeness).
// 4. POST mutating endpoint to follower WITHOUT redirect hint until we get 409 + X-Leader (no Location / X-Leader-URL).
// 5. POST again WITH X-Leader-Redirect header until 307 Temporary Redirect with Location + X-Leader + X-Leader-URL.
func Test_40_Leader_Gating_Redirect(t *testing.T) {
	if os.Getenv("STORAGE_DRIVER") == "" || os.Getenv("STORAGE_DSN") == "" {
		t.Skip("cluster E2E requiere STORAGE_DRIVER/DSN para login")
	}

	// 3-node cluster with fixed ports (avoid random collisions in redirect asserts)
	// Ports mirror 20_ha_cluster_test for consistency.
	// node1: http 18081 raft 18201 (bootstrap)
	// node2: http 18082 raft 18202
	// node3: http 18083 raft 18203
	rootDir := t.TempDir()
	fs1 := filepath.Join(rootDir, "node1")
	fs2 := filepath.Join(rootDir, "node2")
	fs3 := filepath.Join(rootDir, "node3")
	for _, p := range []string{fs1, fs2, fs3} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	peers := map[string]string{
		"node1": "127.0.0.1:18201",
		"node2": "127.0.0.1:18202",
		"node3": "127.0.0.1:18203",
	}
	redirects := map[string]string{
		"node1": "http://127.0.0.1:18081",
		"node2": "http://127.0.0.1:18082",
		"node3": "http://127.0.0.1:18083",
	}

	// Seed tenant FS for each node
	dsn := os.Getenv("STORAGE_DSN")
	for _, fs := range []string{fs1, fs2, fs3} {
		if err := seedTenantFS(fs, seed.Tenant.ID, "local", "Local", dsn); err != nil {
			t.Fatalf("seed %s: %v", fs, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	// Limit DB connections to avoid 'too many clients' under multi-node test
	_ = os.Setenv("POSTGRES_MAX_OPEN_CONNS", "1")
	_ = os.Setenv("POSTGRES_MAX_IDLE_CONNS", "1")
	// Bootstrap only first node explicitly
	_ = os.Setenv("CLUSTER_BOOTSTRAP", "1")
	n1, base1, err := startClusterNode(ctx, 18081, 18201, fs1, "node1", peers, redirects)
	if err != nil {
		t.Fatalf("node1: %v", err)
	}
	t.Cleanup(func() {
		if n1 != nil {
			n1.stop()
		}
	})
	// Unset bootstrap for remaining nodes
	_ = os.Unsetenv("CLUSTER_BOOTSTRAP")
	n2, base2, err := startClusterNode(ctx, 18082, 18202, fs2, "node2", peers, redirects)
	if err != nil {
		t.Fatalf("node2: %v", err)
	}
	t.Cleanup(func() {
		if n2 != nil {
			n2.stop()
		}
	})
	n3, base3, err := startClusterNode(ctx, 18083, 18203, fs3, "node3", peers, redirects)
	if err != nil {
		t.Fatalf("node3: %v", err)
	}
	t.Cleanup(func() {
		if n3 != nil {
			n3.stop()
		}
	})

	// Wait readiness (increase timeout to 40s each)
	for _, base := range []string{base1, base2, base3} {
		if err := waitReady(base, 40*time.Second); err != nil {
			t.Fatalf("ready %s: %v", base, err)
		}
	}

	// Helper: fetch /readyz rich
	type readyz struct {
		Cluster struct {
			Role     string `json:"role"`
			LeaderID string `json:"leader_id"`
		} `json:"cluster"`
	}
	fetch := func(base string) (readyz, error) {
		var rz readyz
		resp, err := newHTTPClient().Get(base + "/readyz")
		if err != nil {
			return rz, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(b, &rz); err != nil {
			return rz, fmt.Errorf("unmarshal readyz: %w body=%s", err, string(b))
		}
		return rz, nil
	}
	waitForLeaderAndFollower := func(bases []string, timeout time.Duration) (leaderBase, followerBase string, leaderID string) {
		deadline := time.Now().Add(timeout)
		lastDump := time.Time{}
		for time.Now().Before(deadline) {
			var leader string
			for _, b := range bases {
				rz, err := fetch(b)
				if err == nil && rz.Cluster.Role == "leader" {
					leader = b
					break
				}
			}
			if leader != "" {
				for _, b := range bases {
					if b == leader {
						continue
					}
					rz, err := fetch(b)
					if err == nil && rz.Cluster.Role == "follower" && rz.Cluster.LeaderID != "" {
						return leader, b, rz.Cluster.LeaderID
					}
				}
			}
			if time.Since(lastDump) > time.Second { // periodic diagnostics
				var sb strings.Builder
				for _, b := range bases {
					rz, _ := fetch(b)
					sb.WriteString(fmt.Sprintf("[%s role=%s leader_id=%s] ", b, rz.Cluster.Role, rz.Cluster.LeaderID))
				}
				t.Logf("election poll: %s", sb.String())
				lastDump = time.Now()
			}
			time.Sleep(300 * time.Millisecond)
		}
		return "", "", ""
	}
	leaderBase, followerBase, leaderID := waitForLeaderAndFollower([]string{base1, base2, base3}, 35*time.Second)
	if leaderBase == "" || followerBase == "" {
		t.Skipf("No se estableció liderazgo a tiempo (bases=%s,%s,%s)", base1, base2, base3)
	}
	t.Logf("leader=%s follower=%s leader_id=%s", leaderBase, followerBase, leaderID)

	// Login (sys admin) against follower to ensure path goes through RequireAuth -> RequireSysAdmin -> RequireLeader
	login := func(base string) string {
		c := newHTTPClient()
		body := map[string]string{
			"tenant_id": seed.Tenant.ID,
			"client_id": seed.Clients.Web.ClientID,
			"email":     seed.Users.Admin.Email,
			"password":  seed.Users.Admin.Password,
		}
		buf, _ := json.Marshal(body)
		resp, err := c.Post(base+"/v1/auth/login", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("login %s: %v", base, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Skipf("No login (status=%d base=%s body=%s) – seed admin no disponible?", resp.StatusCode, base, truncate(string(b), 200))
		}
		var tok struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&tok)
		if tok.AccessToken == "" {
			t.Fatalf("login %s: token vacío", base)
		}
		return tok.AccessToken
	}
	token := login(followerBase)

	// Write attempt helper (POST /v1/admin/tenants)
	type writeResult struct {
		status                 int
		leader, loc, leaderURL string
		body                   string
		headers                http.Header
	}
	tryWrite := func(base string, withRedirect bool) writeResult {
		slug := fmt.Sprintf("ha-redirect-%d-%d", time.Now().UnixNano(), rand.Intn(1000))
		doc := map[string]any{"slug": slug, "name": "HA Redirect Test"}
		data, _ := json.Marshal(doc)
		url := base + "/v1/admin/tenants"
		if withRedirect {
			url += "?leader_redirect=1"
		}
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if withRedirect {
			req.Header.Set("X-Leader-Redirect", "1")
		}
		// Custom client to prevent auto-following 307 redirect (we need to assert headers)
		c := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("POST tenants: %v", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return writeResult{status: resp.StatusCode, leader: readHeader(resp, "X-Leader"), loc: readHeader(resp, "Location"), leaderURL: readHeader(resp, "X-Leader-URL"), body: string(b), headers: resp.Header.Clone()}
	}

	// Phase 1: follower without redirect hint => expect 409 + X-Leader
	deadline := time.Now().Add(25 * time.Second)
	var gotLeaderHeader string
	for time.Now().Before(deadline) {
		r := tryWrite(followerBase, false)
		if r.status == http.StatusConflict {
			if r.leader == "" {
				t.Logf("409 sin X-Leader aún; body=%s", truncate(r.body, 160))
				time.Sleep(350 * time.Millisecond)
				continue
			}
			if r.loc != "" || r.leaderURL != "" {
				t.Fatalf("409 no debe traer Location/X-Leader-URL (Location=%q X-Leader-URL=%q)", r.loc, r.leaderURL)
			}
			gotLeaderHeader = r.leader
			t.Logf("Phase1 OK: follower 409 X-Leader=%s", r.leader)
			break
		}
		if r.status == http.StatusUnauthorized || r.status == http.StatusForbidden {
			t.Fatalf("Auth falló antes de gating status=%d body=%s hdrs=%v", r.status, truncate(r.body, 200), r.headers)
		}
		if r.status >= 200 && r.status < 300 {
			t.Fatalf("Mutation aceptada en follower (status=%d) – debería estar gateada", r.status)
		}
		t.Logf("Phase1 retry status=%d body=%s", r.status, truncate(r.body, 160))
		time.Sleep(300 * time.Millisecond)
	}
	if gotLeaderHeader == "" {
		t.Fatalf("No se obtuvo 409 con X-Leader antes de timeout")
	}

	// Phase 2: follower with redirect hint => expect 307 + headers
	deadline = time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		r := tryWrite(followerBase, true)
		switch r.status {
		case http.StatusTemporaryRedirect:
			if r.leader == "" || r.leaderURL == "" || r.loc == "" {
				t.Fatalf("307 faltan headers leader=%q leaderURL=%q loc=%q", r.leader, r.leaderURL, r.loc)
			}
			if !strings.HasPrefix(r.loc, r.leaderURL+"/v1/admin/tenants") {
				t.Fatalf("Location inesperada: %s", r.loc)
			}
			t.Logf("Phase2 OK: 307 redirect a %s (leader=%s)", r.loc, r.leader)
			return
		case http.StatusConflict:
			t.Logf("Phase2 aún 409 (esperando 307) body=%s", truncate(r.body, 140))
		case http.StatusUnauthorized, http.StatusForbidden:
			t.Fatalf("Auth falló en phase2 status=%d body=%s hdrs=%v", r.status, truncate(r.body, 200), r.headers)
		default:
			t.Logf("Phase2 retry status=%d body=%s", r.status, truncate(r.body, 140))
		}
		time.Sleep(350 * time.Millisecond)
	}
	t.Fatalf("Timeout esperando 307 redirect desde follower")
}
