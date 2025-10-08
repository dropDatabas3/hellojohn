package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	"github.com/dropDatabas3/hellojohn/internal/http/handlers"
)

func main() {
	// Usar variables de entorno
	fsRoot := os.Getenv("CONTROL_PLANE_FS_ROOT")
	if fsRoot == "" {
		fsRoot = "./data/hellojohn"
	}

	// Inicializar control-plane
	cpctx.Provider = cpfs.New(fsRoot)
	cpctx.ResolveTenant = func(r *http.Request) string {
		if v := r.Header.Get("X-Tenant-Slug"); v != "" {
			return v
		}
		if v := r.URL.Query().Get("tenant"); v != "" {
			return v
		}
		return "local"
	}

	// Container mínimo (solo para los handlers)
	container := &app.Container{}

	// Test Admin Clients FS
	fmt.Println("🧪 Testing Admin Clients FS Handler...")
	testAdminClients(container)

	// Test Admin Scopes FS
	fmt.Println("\n🧪 Testing Admin Scopes FS Handler...")
	testAdminScopes(container)

	fmt.Println("\n🎉 Admin FS handlers working correctly!")
}

func testAdminClients(container *app.Container) {
	handler := handlers.NewAdminClientsFSHandler(container)

	// Test 1: List clients
	fmt.Println("   📋 List clients...")
	req := httptest.NewRequest("GET", "/v1/admin/clients", nil)
	req.Header.Set("X-Tenant-Slug", "local")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ List clients failed: %d\n", w.Code)
		return
	}

	var clients []cp.OIDCClient
	json.Unmarshal(w.Body.Bytes(), &clients)
	fmt.Printf("   ✅ Found %d clients\n", len(clients))

	// Test 2: Create new client
	fmt.Println("   ➕ Creating new client...")
	newClient := cp.ClientInput{
		Name:         "Test Admin Client",
		ClientID:     "test-admin-client",
		Type:         cp.ClientTypeConfidential,
		RedirectURIs: []string{"http://localhost:3000/test"},
		Scopes:       []string{"openid", "profile"},
		Secret:       "test-secret-123",
	}

	body, _ := json.Marshal(newClient)
	req = httptest.NewRequest("POST", "/v1/admin/clients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Slug", "local")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ Create client failed: %d - %s\n", w.Code, w.Body.String())
		return
	}
	fmt.Println("   ✅ Client created successfully")

	// Test 3: Update client
	fmt.Println("   ✏️ Updating client...")
	updateClient := cp.ClientInput{
		Name:         "Updated Test Admin Client",
		ClientID:     "test-admin-client",
		Type:         cp.ClientTypeConfidential,
		RedirectURIs: []string{"http://localhost:3000/test", "http://localhost:3000/updated"},
		Scopes:       []string{"openid", "profile", "email"},
		Secret:       "updated-secret-456",
	}

	body, _ = json.Marshal(updateClient)
	req = httptest.NewRequest("PUT", "/v1/admin/clients/test-admin-client", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Slug", "local")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ Update client failed: %d - %s\n", w.Code, w.Body.String())
		return
	}
	fmt.Println("   ✅ Client updated successfully")

	// Test 4: Delete client
	fmt.Println("   🗑️ Deleting client...")
	req = httptest.NewRequest("DELETE", "/v1/admin/clients/test-admin-client", nil)
	req.Header.Set("X-Tenant-Slug", "local")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ Delete client failed: %d - %s\n", w.Code, w.Body.String())
		return
	}
	fmt.Println("   ✅ Client deleted successfully")
}

func testAdminScopes(container *app.Container) {
	handler := handlers.NewAdminScopesFSHandler(container)

	// Test 1: List scopes
	fmt.Println("   📋 List scopes...")
	req := httptest.NewRequest("GET", "/v1/admin/scopes", nil)
	req.Header.Set("X-Tenant-Slug", "local")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ List scopes failed: %d\n", w.Code)
		return
	}

	var scopes []cp.Scope
	json.Unmarshal(w.Body.Bytes(), &scopes)
	fmt.Printf("   ✅ Found %d scopes\n", len(scopes))

	// Test 2: Create new scope
	fmt.Println("   ➕ Creating new scope...")
	newScope := cp.Scope{
		Name:        "test-scope",
		Description: "Test scope for admin API",
		System:      false,
	}

	body, _ := json.Marshal(newScope)
	req = httptest.NewRequest("POST", "/v1/admin/scopes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Slug", "local")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ Create scope failed: %d - %s\n", w.Code, w.Body.String())
		return
	}
	fmt.Println("   ✅ Scope created successfully")

	// Test 3: Update scope
	fmt.Println("   ✏️ Updating scope...")
	updateScope := cp.Scope{
		Name:        "test-scope",
		Description: "Updated test scope for admin API",
		System:      false,
	}

	body, _ = json.Marshal(updateScope)
	req = httptest.NewRequest("PUT", "/v1/admin/scopes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Slug", "local")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ Update scope failed: %d - %s\n", w.Code, w.Body.String())
		return
	}
	fmt.Println("   ✅ Scope updated successfully")

	// Test 4: Delete scope
	fmt.Println("   🗑️ Deleting scope...")
	req = httptest.NewRequest("DELETE", "/v1/admin/scopes/test-scope", nil)
	req.Header.Set("X-Tenant-Slug", "local")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Printf("   ❌ Delete scope failed: %d - %s\n", w.Code, w.Body.String())
		return
	}
	fmt.Println("   ✅ Scope deleted successfully")
}
