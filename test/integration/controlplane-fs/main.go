package main

import (
	"context"
	"fmt"
	"log"
	"os"

	fs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
)

func main() {
	// usar directamente el path de las variables de entorno
	fsRoot := os.Getenv("CONTROL_PLANE_FS_ROOT")
	if fsRoot == "" {
		fsRoot = "./data/hellojohn"
	}

	// crear provider del control-plane
	provider := fs.New(fsRoot)
	ctx := context.Background()

	// listar tenants
	tenants, err := provider.ListTenants(ctx)
	if err != nil {
		log.Fatalf("Error listing tenants: %v", err)
	}

	fmt.Printf("🏢 Found %d tenant(s):\n", len(tenants))
	for _, tenant := range tenants {
		fmt.Printf("   - %s (%s) - %s\n", tenant.Name, tenant.Slug, tenant.ID)

		// listar clients del tenant
		clients, err := provider.ListClients(ctx, tenant.Slug)
		if err != nil {
			fmt.Printf("     Error listing clients: %v\n", err)
			continue
		}

		fmt.Printf("     📱 Clients (%d):\n", len(clients))
		for _, client := range clients {
			fmt.Printf("        - %s (%s) - Type: %s\n", client.Name, client.ClientID, client.Type)
			fmt.Printf("          RedirectURIs: %v\n", client.RedirectURIs)
			fmt.Printf("          Scopes: %v\n", client.Scopes)

			// test descifrado de secreto
			if client.Type == "confidential" {
				secret, err := provider.DecryptClientSecret(ctx, tenant.Slug, client.ClientID)
				if err != nil {
					fmt.Printf("          🔑 Secret: ERROR decrypting - %v\n", err)
				} else {
					fmt.Printf("          🔑 Secret: %s\n", secret)
				}
			}
		}

		// listar scopes del tenant
		scopes, err := provider.ListScopes(ctx, tenant.Slug)
		if err != nil {
			fmt.Printf("     Error listing scopes: %v\n", err)
			continue
		}

		fmt.Printf("     🎯 Scopes (%d):\n", len(scopes))
		for _, scope := range scopes {
			systemFlag := ""
			if scope.System {
				systemFlag = " [SYSTEM]"
			}
			fmt.Printf("        - %s: %s%s\n", scope.Name, scope.Description, systemFlag)
		}
		fmt.Println()
	}

	fmt.Println("✅ Control-plane FS provider working correctly!")
}
