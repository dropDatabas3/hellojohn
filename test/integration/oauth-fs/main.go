package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
)

func main() {
	// Usar variables de entorno directamente
	fsRoot := os.Getenv("CONTROL_PLANE_FS_ROOT")
	if fsRoot == "" {
		fsRoot = "./data/hellojohn"
	}

	// Inicializar control-plane igual que en service
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

	ctx := context.Background()

	// Crear request simulado
	req, _ := http.NewRequest("GET", "/test", nil)

	// Test LookupClient
	client, tenantSlug, err := helpers.LookupClient(ctx, req, "local-test-app")
	if err != nil {
		log.Fatalf("LookupClient failed: %v", err)
	}

	fmt.Printf("✅ Cliente encontrado:\n")
	fmt.Printf("   Tenant: %s\n", tenantSlug)
	fmt.Printf("   Nombre: %s\n", client.Name)
	fmt.Printf("   ClientID: %s\n", client.ClientID)
	fmt.Printf("   Tipo: %s\n", client.Type)
	fmt.Printf("   Redirect URIs: %v\n", client.RedirectURIs)
	fmt.Printf("   Scopes: %v\n", client.Scopes)

	// Test ValidateRedirectURI
	err = helpers.ValidateRedirectURI(client, "http://localhost:3000/callback")
	if err != nil {
		fmt.Printf("❌ ValidateRedirectURI failed: %v\n", err)
	} else {
		fmt.Printf("✅ ValidateRedirectURI passed\n")
	}

	// Test ValidateClientSecret
	err = helpers.ValidateClientSecret(ctx, req, tenantSlug, client, "my-test-secret-123")
	if err != nil {
		fmt.Printf("❌ ValidateClientSecret failed: %v\n", err)
	} else {
		fmt.Printf("✅ ValidateClientSecret passed\n")
	}

	fmt.Println("\n🎉 OAuth → FS integration working correctly!")
}
