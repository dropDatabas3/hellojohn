package controlplane

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
)

func TestFSProvider_TenantCRUD(t *testing.T) {
	// crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "hellojohn-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// configurar secretbox con key temporal
	os.Setenv("SECRETBOX_MASTER_KEY", "e3wlUfaN91WoNvHa9aB47ARoAz1DusF2I+hV7Uyz/wU=")
	defer os.Unsetenv("SECRETBOX_MASTER_KEY")

	provider := cpfs.New(tmpDir)
	ctx := context.Background()

	// Test: listar tenants vacío
	tenants, err := provider.ListTenants(ctx)
	if err != nil {
		t.Fatalf("ListTenants failed: %v", err)
	}
	if len(tenants) != 0 {
		t.Fatalf("Expected 0 tenants, got %d", len(tenants))
	}

	// Test: crear tenant
	tenant := &cp.Tenant{
		ID:   "test-tenant-id",
		Name: "Test Tenant",
		Slug: "test-tenant",
		Settings: cp.TenantSettings{
			BrandColor: "#ff0000",
			LogoURL:    "https://example.com/logo.png",
		},
	}

	err = provider.UpsertTenant(ctx, tenant)
	if err != nil {
		t.Fatalf("UpsertTenant failed: %v", err)
	}

	// verificar que se creó el archivo
	expectedPath := filepath.Join(tmpDir, "tenants", "test-tenant", "tenant.yaml")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Tenant file was not created: %s", expectedPath)
	}

	// Test: obtener tenant
	retrieved, err := provider.GetTenantBySlug(ctx, "test-tenant")
	if err != nil {
		t.Fatalf("GetTenantBySlug failed: %v", err)
	}
	if retrieved.Name != "Test Tenant" {
		t.Fatalf("Expected name 'Test Tenant', got %s", retrieved.Name)
	}
	if retrieved.Settings.BrandColor != "#ff0000" {
		t.Fatalf("Expected brand color '#ff0000', got %s", retrieved.Settings.BrandColor)
	}

	// Test: listar tenants (debe devolver 1)
	tenants, err = provider.ListTenants(ctx)
	if err != nil {
		t.Fatalf("ListTenants failed: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("Expected 1 tenant, got %d", len(tenants))
	}

	t.Logf("✅ Tenant CRUD tests passed")
}

func TestFSProvider_ClientCRUD(t *testing.T) {
	// crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "hellojohn-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// configurar secretbox con key temporal
	os.Setenv("SECRETBOX_MASTER_KEY", "e3wlUfaN91WoNvHa9aB47ARoAz1DusF2I+hV7Uyz/wU=")
	defer os.Unsetenv("SECRETBOX_MASTER_KEY")

	provider := cpfs.New(tmpDir)
	ctx := context.Background()

	// crear tenant primero
	tenant := &cp.Tenant{
		ID:   "test-tenant-id",
		Name: "Test Tenant",
		Slug: "test-tenant",
	}
	err = provider.UpsertTenant(ctx, tenant)
	if err != nil {
		t.Fatalf("UpsertTenant failed: %v", err)
	}

	// Test: crear client confidencial
	clientInput := cp.ClientInput{
		Name:         "Test App",
		ClientID:     "test-app-client",
		Type:         cp.ClientTypeConfidential,
		RedirectURIs: []string{"https://example.com/callback"},
		Scopes:       []string{"openid", "profile", "email"},
		Secret:       "my-super-secret-password",
	}

	client, err := provider.UpsertClient(ctx, "test-tenant", clientInput)
	if err != nil {
		t.Fatalf("UpsertClient failed: %v", err)
	}

	if client.Name != "Test App" {
		t.Fatalf("Expected name 'Test App', got %s", client.Name)
	}
	if client.Type != cp.ClientTypeConfidential {
		t.Fatalf("Expected type 'confidential', got %s", client.Type)
	}
	if client.SecretEnc == "" {
		t.Fatal("Expected encrypted secret, got empty string")
	}

	// Test: descifrar secret
	decryptedSecret, err := provider.DecryptClientSecret(ctx, "test-tenant", "test-app-client")
	if err != nil {
		t.Fatalf("DecryptClientSecret failed: %v", err)
	}
	if decryptedSecret != "my-super-secret-password" {
		t.Fatalf("Expected secret 'my-super-secret-password', got %s", decryptedSecret)
	}

	// Test: obtener client
	retrieved, err := provider.GetClient(ctx, "test-tenant", "test-app-client")
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if retrieved.Name != "Test App" {
		t.Fatalf("Expected name 'Test App', got %s", retrieved.Name)
	}

	// Test: listar clients
	clients, err := provider.ListClients(ctx, "test-tenant")
	if err != nil {
		t.Fatalf("ListClients failed: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}

	t.Logf("✅ Client CRUD tests passed")
}

func TestFSProvider_ScopeCRUD(t *testing.T) {
	// crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "hellojohn-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	provider := cpfs.New(tmpDir)
	ctx := context.Background()

	// crear tenant primero
	tenant := &cp.Tenant{
		ID:   "test-tenant-id",
		Name: "Test Tenant",
		Slug: "test-tenant",
	}
	err = provider.UpsertTenant(ctx, tenant)
	if err != nil {
		t.Fatalf("UpsertTenant failed: %v", err)
	}

	// Test: crear scope
	scope := cp.Scope{
		Name:        "admin",
		Description: "Administrative access",
		System:      false,
	}

	err = provider.UpsertScope(ctx, "test-tenant", scope)
	if err != nil {
		t.Fatalf("UpsertScope failed: %v", err)
	}

	// Test: listar scopes
	scopes, err := provider.ListScopes(ctx, "test-tenant")
	if err != nil {
		t.Fatalf("ListScopes failed: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("Expected 1 scope, got %d", len(scopes))
	}
	if scopes[0].Name != "admin" {
		t.Fatalf("Expected scope name 'admin', got %s", scopes[0].Name)
	}

	t.Logf("✅ Scope CRUD tests passed")
}

func TestFSProvider_Validations(t *testing.T) {
	provider := cpfs.New("/tmp/test")

	// Test ClientID validation
	if !provider.ValidateClientID("my-app-123") {
		t.Fatal("Expected valid clientID to pass")
	}
	if provider.ValidateClientID("My App!") {
		t.Fatal("Expected invalid clientID to fail")
	}

	// Test Redirect URI validation
	if !provider.ValidateRedirectURI("https://example.com/callback") {
		t.Fatal("Expected valid https URI to pass")
	}
	if !provider.ValidateRedirectURI("http://localhost:3000/callback") {
		t.Fatal("Expected valid localhost URI to pass")
	}
	if provider.ValidateRedirectURI("http://example.com/callback") {
		t.Fatal("Expected invalid http URI to fail")
	}

	t.Logf("✅ Validation tests passed")
}

// Helper: crear datos de ejemplo para pruebas manuales
func TestCreateSampleData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sample data creation in short mode")
	}

	// configurar secretbox
	os.Setenv("SECRETBOX_MASTER_KEY", "e3wlUfaN91WoNvHa9aB47ARoAz1DusF2I+hV7Uyz/wU=")
	defer os.Unsetenv("SECRETBOX_MASTER_KEY")

	// usar directorio temporal para no crear basura
	tmpDir, err := os.MkdirTemp("", "hellojohn-sample-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	provider := cpfs.New(tmpDir)
	ctx := context.Background()

	// crear tenant local
	tenant := &cp.Tenant{
		ID:   "b7268f99-606d-469f-9f77-aaaaaaaaaaaa",
		Name: "Local Tenant",
		Slug: "local",
		Settings: cp.TenantSettings{
			LogoURL:    "",
			BrandColor: "#00bcd4",
		},
	}

	if err := provider.UpsertTenant(ctx, tenant); err != nil {
		t.Fatalf("Failed to create sample tenant: %v", err)
	}

	// crear scopes básicos
	scopes := []cp.Scope{
		{Name: "openid", Description: "OpenID Connect", System: true},
		{Name: "profile", Description: "Profile information", System: true},
		{Name: "email", Description: "Email address", System: true},
		{Name: "admin", Description: "Administrative access", System: false},
	}

	for _, scope := range scopes {
		err := provider.UpsertScope(ctx, "local", scope)
		if err != nil {
			t.Fatalf("Failed to create scope %s: %v", scope.Name, err)
		}
	}

	// crear client de ejemplo
	clientInput := cp.ClientInput{
		Name:         "Local Test App",
		ClientID:     "local-test-app",
		Type:         cp.ClientTypeConfidential,
		RedirectURIs: []string{"http://localhost:3000/callback", "https://example.com/callback"},
		Scopes:       []string{"openid", "profile", "email", "admin"},
		Secret:       "my-test-secret-123",
	}

	client, err := provider.UpsertClient(ctx, "local", clientInput)
	if err != nil {
		t.Fatalf("Failed to create sample client: %v", err)
	}

	// mostrar el secreto cifrado para verificación
	t.Logf("📁 Sample data created in: %s", tmpDir)
	t.Logf("🔑 Client secret encrypted: %s", client.SecretEnc)

	// verificar descifrado
	decrypted, _ := provider.DecryptClientSecret(ctx, "local", "local-test-app")
	t.Logf("🔓 Client secret decrypted: %s", decrypted)
}
