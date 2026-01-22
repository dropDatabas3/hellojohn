package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/bootstrap"
	v2server "github.com/dropDatabas3/hellojohn/internal/http/v2/server"
	"github.com/joho/godotenv"

	// CRITICAL: Import adapters to register them via init()
	_ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/dal"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️  No .env file found or error loading it: %v", err)
		log.Println("   Continuing with system environment variables...")
	}

	ctx := context.Background()

	v2Addr := os.Getenv("V2_SERVER_ADDR")
	if v2Addr == "" {
		v2Addr = ":8082" // Default separate port for V2 testing
	}

	log.Printf("🚀 Starting HelloJohn V2 Server on %s", v2Addr)

	// Build V2 handler and dependencies
	v2h, v2cleanup, dal, err := v2server.BuildV2HandlerWithDeps()
	if err != nil {
		log.Fatalf("❌ V2 wiring failed: %v", err)
	}
	defer func() {
		if err := v2cleanup(); err != nil {
			log.Printf("⚠️  V2 cleanup error: %v", err)
		}
	}()

	// Admin Bootstrap: Check if admin users exist, prompt if needed
	if bootstrap.ShouldRunBootstrap(ctx, dal) {
		log.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("         ADMIN BOOTSTRAP REQUIRED")
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if err := bootstrap.CheckAndCreateAdmin(ctx, bootstrap.AdminBootstrapConfig{
			DAL:        dal,
			TenantSlug: getDefaultTenant(),
		}); err != nil {
			log.Printf("⚠️  Admin bootstrap failed: %v", err)
			log.Println("   You can create an admin user later using: ./hellojohn admin:create")
		}

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	}

	log.Printf("✅ V2 Server ready at %s", v2Addr)
	log.Println("\n📋 Available endpoints:")
	log.Println("   • Health: GET /readyz")
	log.Println("   • Login:  POST /v2/auth/login")
	log.Println("   • OIDC:   GET /.well-known/openid-configuration")
	log.Println("   • Admin:  GET /v2/admin/tenants (requires auth)")
	log.Println()

	srv := &http.Server{
		Addr:         v2Addr,
		Handler:      v2h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ V2 server failed: %v", err)
	}
}

// getDefaultTenant returns the default tenant slug for bootstrap
func getDefaultTenant() string {
	tenant := os.Getenv("SEED_TENANT_SLUG")
	if tenant == "" {
		tenant = "local"
	}
	return tenant
}
