package e2e

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 43 - Raft mTLS: 2 nodes with valid CA + 1 with bad CA should not join or discover leader
func Test_43_Raft_mTLS_BadPeerRejected(t *testing.T) {
	// This test requires openssl to be present to generate ephemeral certs
	root := t.TempDir()
	assetsDir := filepath.Join(root, "raftcerts")
	paths, err := genRaftTestCerts(assetsDir)
	if err != nil {
		t.Skipf("skipping mTLS test: %v", err)
	}

	// Seed minimal FS roots (no DB needed for readiness/cluster election)
	fs1 := filepath.Join(root, "n1")
	fs2 := filepath.Join(root, "n2")
	fs3 := filepath.Join(root, "n3")
	for _, d := range []string{fs1, fs2, fs3} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	peers := map[string]string{
		"n1": "127.0.0.1:19201",
		"n2": "127.0.0.1:19202",
		"n3": "127.0.0.1:19203",
	}
	redirects := map[string]string{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// node1 (bootstrap) and node2 share the same CA/cert set
	_ = os.Setenv("CLUSTER_BOOTSTRAP", "1")
	n1, b1, err := startClusterNodeWithEnv(ctx, 19081, 19201, fs1, "n1", peers, redirects, map[string]string{
		"RAFT_TLS_ENABLE":      "1",
		"RAFT_TLS_CERT_FILE":   paths["node1.crt"],
		"RAFT_TLS_KEY_FILE":    paths["node1.key"],
		"RAFT_TLS_CA_FILE":     paths["ca.crt"],
		"RAFT_TLS_SERVER_NAME": "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("n1: %v", err)
	}
	t.Cleanup(func() {
		if n1 != nil {
			n1.stop()
		}
	})
	_ = os.Unsetenv("CLUSTER_BOOTSTRAP")
	n2, b2, err := startClusterNodeWithEnv(ctx, 19082, 19202, fs2, "n2", peers, redirects, map[string]string{
		"RAFT_TLS_ENABLE":      "1",
		"RAFT_TLS_CERT_FILE":   paths["node2.crt"],
		"RAFT_TLS_KEY_FILE":    paths["node2.key"],
		"RAFT_TLS_CA_FILE":     paths["ca.crt"],
		"RAFT_TLS_SERVER_NAME": "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("n2: %v", err)
	}
	t.Cleanup(func() {
		if n2 != nil {
			n2.stop()
		}
	})

	// bad node3 uses different CA
	n3, b3, err := startClusterNodeWithEnv(ctx, 19083, 19203, fs3, "n3", peers, redirects, map[string]string{
		"RAFT_TLS_ENABLE":      "1",
		"RAFT_TLS_CERT_FILE":   paths["badnode.crt"],
		"RAFT_TLS_KEY_FILE":    paths["badnode.key"],
		"RAFT_TLS_CA_FILE":     paths["badca.crt"],
		"RAFT_TLS_SERVER_NAME": "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("n3: %v", err)
	}
	t.Cleanup(func() {
		if n3 != nil {
			n3.stop()
		}
	})

	for _, base := range []string{b1, b2, b3} {
		if err := waitReady(base, 35*time.Second); err != nil {
			t.Fatalf("ready %s: %v", base, err)
		}
	}

	type rz struct {
		Cluster struct {
			Role, LeaderID string `json:"role" json:"leader_id"`
		} `json:"cluster"`
	}
	fetch := func(base string) rz {
		var out rz
		resp, err := newHTTPClient().Get(base + "/readyz")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			_ = json.Unmarshal(b, &out)
		}
		return out
	}

	// Expect n1/n2 to elect a leader and n3 to have no leader_id (cannot mutually authenticate)
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		r1, r2, r3 := fetch(b1), fetch(b2), fetch(b3)
		if (r1.Cluster.Role == "leader" || r2.Cluster.Role == "leader") && (r1.Cluster.Role == "follower" || r2.Cluster.Role == "follower") {
			// leader/follower among n1,n2 achieved
			if r3.Cluster.LeaderID == "" {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("bad peer appears to have joined or discovered leader")
}
