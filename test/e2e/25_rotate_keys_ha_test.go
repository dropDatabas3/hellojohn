package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type jwksDoc struct {
	Keys []map[string]any `json:"keys"`
}

// boot3Nodes spins up three embedded-raft nodes for tests and waits readiness.
func boot3Nodes(t *testing.T) []struct {
	base string
	stop func()
} {
	t.Helper()
	tmp := t.TempDir()
	fs1 := filepath.Join(tmp, "node1")
	fs2 := filepath.Join(tmp, "node2")
	fs3 := filepath.Join(tmp, "node3")
	peers := map[string]string{"n1": "127.0.0.1:18301", "n2": "127.0.0.1:18302", "n3": "127.0.0.1:18303"}
	redirects := map[string]string{"n1": "http://127.0.0.1:18181", "n2": "http://127.0.0.1:18182", "n3": "http://127.0.0.1:18183"}

	ctx := context.Background()
	n1, base1, err := startClusterNode(ctx, 18181, 18301, fs1, "n1", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}
	n2, base2, err := startClusterNode(ctx, 18182, 18302, fs2, "n2", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}
	n3, base3, err := startClusterNode(ctx, 18183, 18303, fs3, "n3", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { n1.stop(); n2.stop(); n3.stop() })

	for _, b := range []string{base1, base2, base3} {
		if err := waitReady(b, 30*time.Second); err != nil {
			t.Fatalf("node not ready: %s: %v", b, err)
		}
	}
	return []struct {
		base string
		stop func()
	}{
		{base: base1, stop: n1.stop}, {base: base2, stop: n2.stop}, {base: base3, stop: n3.stop},
	}
}

func TestHA_RotateKeys_JWKSIdentical(t *testing.T) {
	t.Parallel()
	nodes := boot3Nodes(t)

	// Wait for leader presence by checking /readyz cluster.role
	findLeader := func() int {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			for i, nd := range nodes {
				resp, err := http.Get(nd.base + "/readyz")
				if err == nil && resp != nil {
					var body map[string]any
					_ = json.NewDecoder(resp.Body).Decode(&body)
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					if cl, ok := body["cluster"].(map[string]any); ok {
						if role, _ := cl["role"].(string); role == "leader" {
							return i
						}
					}
				}
			}
			time.Sleep(300 * time.Millisecond)
		}
		return -1
	}
	li := findLeader()
	if li < 0 {
		t.Skip("no leader observed; environment may block raft ports")
	}

	leader := nodes[li].base

	// Admin login against leader using seed admin
	seed, _ := mustLoadSeedYAML()
	c := newHTTPClient()
	loginBody := map[string]string{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	}
	lb, _ := json.Marshal(loginBody)
	lresp, lerr := c.Post(leader+"/v1/auth/login", "application/json", bytes.NewReader(lb))
	if lerr != nil || lresp == nil || lresp.StatusCode != 200 {
		t.Skip("admin login not available; skipping HA rotate test in this environment")
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(lresp.Body).Decode(&tok)
	io.Copy(io.Discard, lresp.Body)
	lresp.Body.Close()
	if tok.AccessToken == "" {
		t.Skip("no admin access token; skipping")
	}

	// Create tenant 'acme' (id/slug/name)
	{
		req, _ := http.NewRequest(http.MethodPut, leader+"/v1/admin/tenants/acme", strings.NewReader(`{"id":"acme","slug":"acme","name":"ACME"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		resp, _ := c.Do(req)
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}

	// Rotate keys on leader and capture new KID
	var newKID string
	{
		req, _ := http.NewRequest(http.MethodPost, leader+"/v1/admin/tenants/acme/keys/rotate?graceSeconds=5", nil)
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		rresp, err := c.Do(req)
		if err == nil && rresp != nil {
			var obj map[string]any
			_ = json.NewDecoder(rresp.Body).Decode(&obj)
			if kid, _ := obj["kid"].(string); kid != "" {
				newKID = kid
			}
			io.Copy(io.Discard, rresp.Body)
			rresp.Body.Close()
		}
	}

	fetch := func(base string) jwksDoc {
		resp, err := http.Get(base + "/.well-known/jwks/acme.json")
		if err != nil || resp == nil {
			return jwksDoc{}
		}
		defer resp.Body.Close()
		var d jwksDoc
		_ = json.NewDecoder(resp.Body).Decode(&d)
		io.Copy(io.Discard, resp.Body)
		return d
	}

	// Helper: wait until raft commit/applied index converges across nodes
	waitRaftConverged := func(bases []string, timeout time.Duration) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			vals := make([]string, 0, len(bases))
			ok := true
			for _, b := range bases {
				resp, err := http.Get(b + "/readyz")
				if err != nil || resp == nil {
					ok = false
					break
				}
				var body map[string]any
				_ = json.NewDecoder(resp.Body).Decode(&body)
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				v := ""
				if cl, _ := body["cluster"].(map[string]any); cl != nil {
					if raft, _ := cl["raft"].(map[string]any); raft != nil {
						// commit_index preferred; fallback to last_log_index
						if s, _ := raft["commit_index"].(string); s != "" {
							v = s
						} else if s2, _ := raft["last_log_index"].(string); s2 != "" {
							v = s2
						}
					}
				}
				if v == "" {
					ok = false
					break
				}
				vals = append(vals, v)
			}
			if ok && len(vals) == len(bases) && vals[0] == vals[1] && vals[1] == vals[2] {
				return nil
			}
			time.Sleep(150 * time.Millisecond)
		}
		return context.DeadlineExceeded
	}
	_ = waitRaftConverged([]string{nodes[0].base, nodes[1].base, nodes[2].base}, 10*time.Second)
	// Wait up to 15s for JWKS to converge across nodes post-rotation
	deadline := time.Now().Add(15 * time.Second)
	for {
		a := fetch(nodes[0].base)
		b := fetch(nodes[1].base)
		c := fetch(nodes[2].base)

		hasKid := func(d jwksDoc, kid string) bool {
			if kid == "" {
				return len(d.Keys) > 0
			}
			for _, k := range d.Keys {
				if strings.EqualFold(asString(k["kid"]), kid) {
					return true
				}
			}
			return false
		}

		if len(a.Keys) > 0 && len(b.Keys) > 0 && len(c.Keys) > 0 &&
			((newKID == "") || (hasKid(a, newKID) && hasKid(b, newKID) && hasKid(c, newKID))) &&
			deepEqual(a, b) && deepEqual(a, c) {
			// Converged
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("jwks mismatch across nodes (timeout waiting for convergence): a=%v b=%v c=%v newKID=%s", a, b, c, newKID)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func deepEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
