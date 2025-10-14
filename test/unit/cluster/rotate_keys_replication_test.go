package cluster_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/hashicorp/raft"
)

// This test simulates leader-generated rotation material propagated via MutationRotateTenantKey
// and ensures JWKS output is identical across two FS roots after applying the same mutation.
func TestRotateTenantKey_Replicated_JWKSIdentical(t *testing.T) {
	// SIGNING_MASTER_KEY must be present; use a 32-byte hex string (64 hex chars)
	os.Setenv("SIGNING_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	defer os.Unsetenv("SIGNING_MASTER_KEY")

	srcRoot, _ := os.MkdirTemp("", "hj-rot-src-*")
	defer os.RemoveAll(srcRoot)
	dstRoot, _ := os.MkdirTemp("", "hj-rot-dst-*")
	defer os.RemoveAll(dstRoot)

	// Providers
	_ = cpfs.New(srcRoot)
	dst := cpfs.New(dstRoot)

	// Seed minimal tenant structure so JWKSForTenant can bootstrap
	if err := os.MkdirAll(filepath.Join(srcRoot, "tenants", "acme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dstRoot, "tenants", "acme"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Leader: create file keystore and rotate once to produce files
	leaderKS, err := jwtx.NewFileSigningKeyStore(filepath.Join(srcRoot, "keys"))
	if err != nil {
		t.Fatalf("leader ks: %v", err)
	}
	pks := jwtx.NewPersistentKeystore(context.Background(), leaderKS)
	if _, _, _, err := pks.ActiveForTenant("acme"); err != nil {
		t.Fatalf("bootstrap active: %v", err)
	}
	if _, err := pks.RotateFor("acme", 5); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// Read exact JSON files from leader
	act, err := os.ReadFile(filepath.Join(srcRoot, "keys", "acme", "active.json"))
	if err != nil {
		t.Fatalf("read active: %v", err)
	}
	var ret []byte
	if b, rerr := os.ReadFile(filepath.Join(srcRoot, "keys", "acme", "retiring.json")); rerr == nil {
		ret = b
	}

	// Apply mutation on follower via FSM
	cpctx.Provider = dst
	fsm := cluster.NewFSM()
	dto := cluster.RotateTenantKeyDTO{ActiveJSON: string(act), RetiringJSON: string(ret), GraceSeconds: 5}
	payload, _ := json.Marshal(dto)
	// Build a real raft.Log value
	m := cluster.Mutation{Type: cluster.MutationRotateTenantKey, TenantSlug: "acme", TsUnix: 1, Payload: payload}
	mbytes, _ := json.Marshal(m)
	log := &raft.Log{Data: mbytes}
	fsm.Apply(log)

	// Compare JWKS between leader and follower
	leaderJ := jwtx.NewPersistentKeystore(context.Background(), leaderKS)
	leaderJWKS, err := leaderJ.JWKSJSONForTenant("acme")
	if err != nil {
		t.Fatalf("leader jwks: %v", err)
	}

	followerKS, _ := jwtx.NewFileSigningKeyStore(filepath.Join(dstRoot, "keys"))
	followerJ := jwtx.NewPersistentKeystore(context.Background(), followerKS)
	followerJWKS, err := followerJ.JWKSJSONForTenant("acme")
	if err != nil {
		t.Fatalf("follower jwks: %v", err)
	}

	if string(leaderJWKS) != string(followerJWKS) {
		t.Fatalf("JWKS mismatch after replicated rotation\nleader=%s\nfollower=%s", leaderJWKS, followerJWKS)
	}
}
