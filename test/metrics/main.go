package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
	httpmetrics "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
)

func main() {
	// Configurar control plane
	cpctx.Provider = fs.New("../data/hellojohn")

	// Crear tenant SQL manager con métricas
	manager, err := tenantsql.New(tenantsql.Config{
		Pool: tenantsql.PoolConfig{
			MaxOpenConns:    15,
			MaxIdleConns:    3,
			ConnMaxLifetime: 30 * time.Minute,
		},
		MigrationsDir: "../migrations/postgres",
		MetricsFunc: httpmetrics.RecordTenantMigration,
	})
	if err != nil {
		log.Fatalf("tenant sql manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Forzar creación de pools para local y staging
	tenants := []string{"local", "staging"}
	for _, slug := range tenants {
		fmt.Printf("Testing tenant: %s\n", slug)
		store, err := manager.GetPG(ctx, slug)
		if err != nil {
			fmt.Printf("Error accessing tenant %s: %v\n", slug, err)
		} else if store != nil {
			fmt.Printf("Successfully created pool for tenant %s\n", slug)
		}
	}

	fmt.Println("Tenant pool creation test completed")
}