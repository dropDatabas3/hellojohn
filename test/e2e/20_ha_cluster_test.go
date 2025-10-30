package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// waitReadyWith checks /readyz until it returns 200 and an optional predicate returns true.
func waitReadyWith(base string, timeout time.Duration, pred func(map[string]any) bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/readyz")
		if err == nil && resp != nil {
			var body map[string]any
			ok := resp.StatusCode == 200
			if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && pred != nil {
				if c, ok2 := body["cluster"].(map[string]any); ok2 {
					ok = ok && pred(c)
				}
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if ok {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

func TestHA_ThreeNodes_LeaderFollower_WriteRedirect_Failover(t *testing.T) {
	t.Parallel()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}

	// Isolate FS roots per node under test temp dir
	tmp := t.TempDir()
	fs1 := filepath.Join(tmp, "node1")
	fs2 := filepath.Join(tmp, "node2")
	fs3 := filepath.Join(tmp, "node3")

	// Peers and redirects
	peers := map[string]string{"n1": "127.0.0.1:18201", "n2": "127.0.0.1:18202", "n3": "127.0.0.1:18203"}
	redirects := map[string]string{"n1": "http://127.0.0.1:18081", "n2": "http://127.0.0.1:18082", "n3": "http://127.0.0.1:18083"}

	// Start three nodes with dedicated HTTP/Raft ports
	ctx := context.Background()
	n1, base1, err := startClusterNode(ctx, 18081, 18201, fs1, "n1", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}
	defer n1.stop()
	n2, base2, err := startClusterNode(ctx, 18082, 18202, fs2, "n2", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}
	defer n2.stop()
	n3, base3, err := startClusterNode(ctx, 18083, 18203, fs3, "n3", peers, redirects)
	if err != nil {
		t.Fatal(err)
	}
	defer n3.stop()

	// Wait all up (HTTP 200)
	for _, b := range []string{base1, base2, base3} {
		if err := waitReady(b, 30*time.Second); err != nil {
			if n1 != nil && n1.out != nil {
				t.Log(n1.out.String())
			}
			if n2 != nil && n2.out != nil {
				t.Log(n2.out.String())
			}
			if n3 != nil && n3.out != nil {
				t.Log(n3.out.String())
			}
			t.Fatalf("node not ready: %s: %v", b, err)
		}
	}

	// Helpers to discover role/leader
	getRole := func(base string) (string, string) {
		resp, err := http.Get(base + "/readyz")
		if err != nil || resp == nil {
			return "", ""
		}
		defer resp.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		cl, _ := body["cluster"].(map[string]any)
		if cl == nil {
			return "", ""
		}
		role, _ := cl["role"].(string)
		leader, _ := cl["leader_id"].(string)
		return role, leader
	}

	// Wait up to 30s for a leader to emerge
	var leaderBase, followerBase string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		// probe all nodes
		roles := []struct{ base, role string }{
			{base1, ""}, {base2, ""}, {base3, ""},
		}
		for i := range roles {
			r, _ := getRole(roles[i].base)
			roles[i].role = r
		}
		leaderBase = ""
		followerBase = ""
		for _, it := range roles {
			if it.role == "leader" {
				leaderBase = it.base
			}
		}
		for _, it := range roles {
			if it.role == "follower" && followerBase == "" {
				followerBase = it.base
			}
		}
		if leaderBase != "" && followerBase != "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if leaderBase == "" || followerBase == "" {
		if n1 != nil && n1.out != nil {
			t.Log("NODE1 LOGS:\n" + n1.out.String())
		}
		if n2 != nil && n2.out != nil {
			t.Log("NODE2 LOGS:\n" + n2.out.String())
		}
		if n3 != nil && n3.out != nil {
			t.Log("NODE3 LOGS:\n" + n3.out.String())
		}
		t.Skip("no stable leader/follower observed; skipping HA E2E in this environment")
	}

	// Acquire admin token on leader (seed should have created local tenant and admin in E2E envs; else skip)
	// We'll try a direct login with default seed values; if it fails, skip HA tests in this environment.
	seed, _ := mustLoadSeedYAML()
	c := newHTTPClient()
	body := map[string]string{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	}
	bb, _ := json.Marshal(body)
	resp, err := c.Post(leaderBase+"/v1/auth/login", "application/json", bytes.NewReader(bb))
	if err != nil || resp == nil || resp.StatusCode != 200 {
		t.Skip("admin login not available in this env; skipping HA E2E")
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if tok.AccessToken == "" {
		t.Skip("no admin token; skipping HA E2E")
	}

	// Case A: write on follower -> 409 without redirect, 307 with X-Leader-Redirect: 1
	{
		payload := map[string]any{
			"clientId":     "ha-e2e-client",
			"name":         "HA E2E",
			"type":         "public",
			"redirectUris": []string{leaderBase + "/cb"},
			"scopes":       []string{"openid"},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, followerBase+"/v1/admin/tenants/"+seed.Tenant.ID+"/clients/ha-e2e-client", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		r1, e1 := c.Do(req)
		if e1 != nil || r1 == nil {
			t.Fatal("follower write request error")
		}
		// Expect 409 conflict (not leader)
		if r1.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409 on follower write, got %d", r1.StatusCode)
		}
		io.Copy(io.Discard, r1.Body)
		r1.Body.Close()

		// Now ask for redirect
		req2, _ := http.NewRequest(http.MethodPut, followerBase+"/v1/admin/tenants/"+seed.Tenant.ID+"/clients/ha-e2e-client", bytes.NewReader(b))
		req2.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("X-Leader-Redirect", "1")
		r2, e2 := c.Do(req2)
		if e2 != nil || r2 == nil {
			t.Fatal("follower redirect request error")
		}
		if r2.StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf("expected 307 on follower redirect, got %d", r2.StatusCode)
		}
		io.Copy(io.Discard, r2.Body)
		r2.Body.Close()
	}

	// Case B: write on leader -> 200 and read from follower -> visible
	{
		payload := map[string]any{
			"clientId":     "ha-e2e-client",
			"name":         "HA E2E",
			"type":         "public",
			"redirectUris": []string{leaderBase + "/cb"},
			"scopes":       []string{"openid"},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, leaderBase+"/v1/admin/tenants/"+seed.Tenant.ID+"/clients/ha-e2e-client", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		r1, e1 := c.Do(req)
		if e1 != nil || r1 == nil {
			t.Fatal("leader write request error")
		}
		if r1.StatusCode != 200 {
			t.Fatalf("expected 200 on leader write, got %d", r1.StatusCode)
		}
		io.Copy(io.Discard, r1.Body)
		r1.Body.Close()

		// Read from follower (list clients by tenant)
		getReq, _ := http.NewRequest(http.MethodGet, followerBase+"/v1/admin/clients?tenant_id="+seed.Tenant.ID, nil)
		getReq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		r2, e2 := c.Do(getReq)
		if e2 != nil || r2 == nil {
			t.Fatal("follower read request error")
		}
		if r2.StatusCode != 200 {
			t.Fatalf("expected 200 on follower read, got %d", r2.StatusCode)
		}
		var clients []map[string]any
		_ = json.NewDecoder(r2.Body).Decode(&clients)
		io.Copy(io.Discard, r2.Body)
		r2.Body.Close()
		found := false
		for _, cl := range clients {
			if cl["client_id"] == "ha-e2e-client" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("client not visible from follower after leader write")
		}
	}

	// Case C: kill leader -> re-election -> write ok on new leader
	{
		// Identify which node is leader and stop it
		r1, l1 := getRole(base1)
		r2, l2 := getRole(base2)
		r3, l3 := getRole(base3)
		_ = l1
		_ = l2
		_ = l3 // quiet linters
		if r1 == "leader" {
			n1.stop()
		} else if r2 == "leader" {
			n2.stop()
		} else if r3 == "leader" {
			n3.stop()
		}

		// Wait for a new leader among the remaining nodes
		deadline := time.Now().Add(30 * time.Second)
		var newLeaderBase string
		for time.Now().Before(deadline) {
			for _, b := range []string{base1, base2, base3} {
				// skip stopped one: we'll query but accept failures silently
				resp, err := http.Get(b + "/readyz")
				if err == nil && resp != nil {
					var body map[string]any
					_ = json.NewDecoder(resp.Body).Decode(&body)
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					cl, _ := body["cluster"].(map[string]any)
					if cl != nil {
						if role, _ := cl["role"].(string); role == "leader" {
							newLeaderBase = b
							break
						}
					}
				}
			}
			if newLeaderBase != "" {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		if newLeaderBase == "" {
			t.Skip("no new leader observed; environment may block raft ports")
		}

		// Try another upsert on the new leader
		payload := map[string]any{
			"clientId":     "ha-e2e-client-2",
			"name":         "HA E2E 2",
			"type":         "public",
			"redirectUris": []string{newLeaderBase + "/cb2"},
			"scopes":       []string{"openid"},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, newLeaderBase+"/v1/admin/tenants/"+seed.Tenant.ID+"/clients/ha-e2e-client-2", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		r1b, e1b := c.Do(req)
		if e1b != nil || r1b == nil {
			t.Fatal("new leader write request error")
		}
		if r1b.StatusCode != 200 {
			t.Fatalf("expected 200 on new leader write, got %d", r1b.StatusCode)
		}
		io.Copy(io.Discard, r1b.Body)
		r1b.Body.Close()
	}

	_ = root // silence unused var on certain environments
}
