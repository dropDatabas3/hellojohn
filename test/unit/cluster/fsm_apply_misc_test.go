package cluster_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	"github.com/hashicorp/raft"
)

func setupFSProvider(t *testing.T) (root string, provider *cpfs.FSProvider) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "hj-fsm-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	os.Setenv("SECRETBOX_MASTER_KEY", "e3wlUfaN91WoNvHa9aB47ARoAz1DusF2I+hV7Uyz/wU=")
	t.Cleanup(func() { os.Unsetenv("SECRETBOX_MASTER_KEY") })
	p := cpfs.New(tmp)
	cpctx.Provider = p
	return tmp, p
}

func applyMutation(t *testing.T, m cluster.Mutation) error {
	data, _ := json.Marshal(m)
	fsm := cluster.NewFSM()
	ret := fsm.Apply(&raft.Log{Data: data})
	if err, ok := ret.(error); ok && err != nil {
		return err
	}
	return nil
}

func TestFSM_Apply_UpsertDeleteTenant_UpdateSettings(t *testing.T) {
	root, provider := setupFSProvider(t)

	// Upsert tenant
	dto := cluster.UpsertTenantDTO{ID: "t1", Name: "Acme", Slug: "acme", Settings: cp.TenantSettings{IssuerMode: cp.IssuerModeGlobal}}
	payload, _ := json.Marshal(dto)
	if err := applyMutation(t, cluster.Mutation{Type: cluster.MutationUpsertTenant, TenantSlug: "acme", TsUnix: 1, Payload: payload}); err != nil {
		t.Fatalf("upsert tenant: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tenants", "acme", "tenant.yaml")); err != nil {
		t.Fatalf("tenant.yaml missing: %v", err)
	}

	// Update settings
	s := cp.TenantSettings{IssuerMode: cp.IssuerModePath}
	sp, _ := json.Marshal(cluster.UpdateTenantSettingsDTO{Settings: s})
	if err := applyMutation(t, cluster.Mutation{Type: cluster.MutationUpdateTenantSettings, TenantSlug: "acme", TsUnix: 2, Payload: sp}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	tnt, err := provider.GetTenantBySlug(context.Background(), "acme")
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if tnt.Settings.IssuerMode != cp.IssuerModePath {
		t.Fatalf("settings not updated deterministically: %+v", tnt.Settings)
	}

	// Delete tenant (soft delete tenant.yaml)
	if err := applyMutation(t, cluster.Mutation{Type: cluster.MutationDeleteTenant, TenantSlug: "acme", TsUnix: 3}); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	// tenant.yaml should be renamed
	if _, err := os.Stat(filepath.Join(root, "tenants", "acme", "tenant.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected tenant.yaml to be moved, err=%v", err)
	}
}

func TestFSM_Apply_UpsertDeleteScope(t *testing.T) {
	_, provider := setupFSProvider(t)
	// create tenant baseline
	_ = provider.UpsertTenant(context.Background(), &cp.Tenant{Slug: "acme", Name: "Acme"})

	// Upsert scope
	sp, _ := json.Marshal(cluster.UpsertScopeDTO{Name: "profile:read", Description: "read profile"})
	if err := applyMutation(t, cluster.Mutation{Type: cluster.MutationUpsertScope, TenantSlug: "acme", TsUnix: 1, Payload: sp}); err != nil {
		t.Fatalf("upsert scope: %v", err)
	}
	scopes, _ := provider.ListScopes(context.Background(), "acme")
	if len(scopes) != 1 || scopes[0].Name != "profile:read" {
		t.Fatalf("unexpected scopes: %+v", scopes)
	}

	// Delete scope
	dp, _ := json.Marshal(cluster.DeleteScopeDTO{Name: "profile:read"})
	if err := applyMutation(t, cluster.Mutation{Type: cluster.MutationDeleteScope, TenantSlug: "acme", TsUnix: 2, Payload: dp}); err != nil {
		t.Fatalf("delete scope: %v", err)
	}
	scopes, _ = provider.ListScopes(context.Background(), "acme")
	if len(scopes) != 0 {
		t.Fatalf("scope not deleted deterministically: %+v", scopes)
	}
}

func TestFSM_Apply_DeleteClient(t *testing.T) {
	_, provider := setupFSProvider(t)
	_ = provider.UpsertTenant(context.Background(), &cp.Tenant{Slug: "acme", Name: "Acme"})
	// Seed one client
	_, _ = provider.UpsertClient(context.Background(), "acme", cp.ClientInput{ClientID: "c1", Name: "C1", Type: cp.ClientTypePublic, RedirectURIs: []string{"http://localhost/cb"}})

	// Delete via FSM
	dp, _ := json.Marshal(cluster.DeleteClientDTO{ClientID: "c1"})
	if err := applyMutation(t, cluster.Mutation{Type: cluster.MutationDeleteClient, TenantSlug: "acme", TsUnix: 2, Payload: dp}); err != nil {
		t.Fatalf("delete client: %v", err)
	}
	cs, _ := provider.ListClients(context.Background(), "acme")
	if len(cs) != 0 {
		t.Fatalf("client not deleted deterministically: %+v", cs)
	}
}
