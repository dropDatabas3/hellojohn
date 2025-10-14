package cluster_test

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
    cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
    "github.com/dropDatabas3/hellojohn/internal/app/cpctx"
    "github.com/dropDatabas3/hellojohn/internal/cluster"
    "github.com/hashicorp/raft"
)

// Test that FSM.Apply(MutationUpsertClient) writes clients.yaml via FS provider
func TestFSM_Apply_UpsertClient_WritesClientsYAML(t *testing.T) {
    tmpDir, err := os.MkdirTemp("", "hj-fsm-*")
    if err != nil { t.Fatal(err) }
    defer os.RemoveAll(tmpDir)

    // Secretbox master key for encryption paths in FS provider
    os.Setenv("SECRETBOX_MASTER_KEY", "e3wlUfaN91WoNvHa9aB47ARoAz1DusF2I+hV7Uyz/wU=")
    defer os.Unsetenv("SECRETBOX_MASTER_KEY")

    // Wire FS provider into global cpctx for FSM usage
    provider := cpfs.New(tmpDir)
    cpctx.Provider = provider

    // Ensure tenant exists
    if err := provider.UpsertTenant(context.Background(), &cp.Tenant{Slug: "local", Name: "Local"}); err != nil {
        t.Fatalf("upsert tenant: %v", err)
    }

    // Build DTO and Mutation
    dto := cluster.UpsertClientDTO{
        Name:         "Test App",
        ClientID:     "test-app",
        Type:         cp.ClientTypePublic,
        RedirectURIs: []string{"http://localhost:3000/callback"},
        Scopes:       []string{"openid", "profile"},
    }
    payload, _ := json.Marshal(dto)
    m := cluster.Mutation{
        Type:       cluster.MutationUpsertClient,
        TenantSlug: "local",
        TsUnix:     1,
        Payload:    payload,
    }
    data, _ := json.Marshal(m)

    fsm := cluster.NewFSM()
    // Directly invoke Apply with a synthetic raft.Log
    // We don't need a real raft index/term for this unit test
    ret := fsm.Apply(&raft.Log{Data: data})
    if err, ok := ret.(error); ok && err != nil {
        t.Fatalf("apply returned error: %v", err)
    }

    // Verify clients.yaml exists and contains our client
    clientsPath := filepath.Join(tmpDir, "tenants", "local", "clients.yaml")
    if _, err := os.Stat(clientsPath); err != nil {
        t.Fatalf("expected clients.yaml to be written, got err: %v", err)
    }

    // Read back via provider
    clients, err := provider.ListClients(context.Background(), "local")
    if err != nil { t.Fatalf("list clients: %v", err) }
    if len(clients) != 1 || clients[0].ClientID != "test-app" {
        t.Fatalf("unexpected clients: %+v", clients)
    }
}

