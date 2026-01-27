package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/bootstrap"
	v2server "github.com/dropDatabas3/hellojohn/internal/http/server"
	"github.com/joho/godotenv"

	_ "github.com/dropDatabas3/hellojohn/internal/store/adapters/dal"
)

func main() {
	log.Println("\n  _   _      _ _           _       _           \n | | | | ___| | | ___     | | ___ | |__  _ __  \n | |_| |/ _ \\ | |/ _ \\    | |/ _ \\| '_ \\| '_ \\ \n |  _  |  __/ | | (_) |  _| | (_) | | | | | | |\n |_| |_|\\___|_|_|\\___/  |___|\\___/|_| |_|_| |_|\n")
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
		log.Println("Continuing with system environment variables...")
	}

	ctx := context.Background()

	v2Addr := os.Getenv("V2_SERVER_ADDR")
	if v2Addr == "" {
		v2Addr = ":8080" // Default separate port for V2 testing
	}

	log.Printf("Starting Server on %s", v2Addr)

	// Build V2 handler and dependencies
	v2h, v2cleanup, dal, err := v2server.BuildV2HandlerWithDeps()
	if err != nil {
		log.Fatalf("Wiring failed: %v", err)
	}
	defer func() {
		if err := v2cleanup(); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}()

	// Admin Bootstrap: Check if admin users exist, prompt if needed
	if bootstrap.ShouldRunBootstrap(ctx, dal) {
		log.Println("\n               ,      \t\n     __  _.-´´` `'-.   \n    /||\\'._ __{}_(\t\n    ||||  |'--.__\\   \t\n    |  L.(   ^_\\^\t   \t\n    \\ .-' |   _ |\t    \n    | |   )\\___/\t    \n    |  \\-'`:._]\t    \n    \\__/;      '-.\t")

		if err := bootstrap.CheckAndCreateAdmin(ctx, bootstrap.AdminBootstrapConfig{
			DAL: dal,
		}); err != nil {
			log.Printf("   Admin bootstrap failed: %v", err)
			log.Println("   You can create an admin user later using: ./hellojohn admin:create")
		}

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	}

	log.Printf("✅ V2 Server ready at %s", v2Addr)

	srv := &http.Server{
		Addr:         v2Addr,
		Handler:      v2h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("    Server failed: %v", err)
	}
}
