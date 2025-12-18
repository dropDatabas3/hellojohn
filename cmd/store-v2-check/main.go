package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/v2"
	_ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/dal" // Importante: registrar adapters
)

func main() {
	fmt.Println("🚀 Store V2 Smoke Test")
	fmt.Println("=======================")

	ctx := context.Background()
	cwd, _ := os.Getwd()
	fmt.Printf("📂 Working Directory: %s\n", cwd)

	// 1. Inicializar Manager
	fmt.Println("\n[1] Inicializando Store Manager...")
	mgr, err := store.NewManager(ctx, store.ManagerConfig{
		FSRoot: "./data", // Asume que se corre desde root del proyecto
		Logger: log.New(os.Stdout, "[STORE] ", 0),
		// Configuración DB tenant default (opcional para test)
		DefaultTenantDB: &store.DBConfig{
			Driver: "postgres",
			DSN:    "postgres://user:pass@localhost:5432/hellojohn?sslmode=disable",
		},
	})
	if err != nil {
		fatal("Error creando manager:", err)
	}
	defer mgr.Close()

	fmt.Printf("✅ Manager iniciado en modo: %s\n", mgr.Mode())

	// 2. Probar Control Plane (FS)
	fmt.Println("\n[2] Probando Control Plane (Tenants)...")
	tenants, err := mgr.ConfigAccess().Tenants().List(ctx)
	if err != nil {
		fatal("Error listando tenants:", err)
	}
	fmt.Printf("✅ %d Tenants encontrados\n", len(tenants))

	for _, t := range tenants {
		fmt.Printf("   - %s (%s)\n", t.Name, t.Slug)
	}

	if len(tenants) == 0 {
		fmt.Println("⚠️ No hay tenants para probar Data Plane. Saliendo.")
		return
	}

	// 3. Probar Data Plane con el primer tenant
	slug := tenants[0].Slug
	testTenant(ctx, mgr, slug)

	fmt.Println("\n✨ SMOKE TEST COMPLETADO EXITOSAMENTE")
}

func testTenant(ctx context.Context, mgr *store.Manager, slug string) {
	fmt.Printf("\n[3] Probando Data Plane para tenant: '%s'...\n", slug)

	// Obtener acceso
	tda, err := mgr.ForTenant(ctx, slug)
	if err != nil {
		fatal("Error obteniendo acceso al tenant:", err)
	}

	fmt.Printf("   Driver: %s\n", tda.Driver())
	fmt.Printf("   HasDB:  %v\n", tda.HasDB())

	// Probar clientes (FS)
	clients, err := tda.Clients().List(ctx, slug, "")
	if err != nil {
		fatal("Error listando clientes:", err)
	}
	fmt.Printf("   ✅ Clients: %d encontrados\n", len(clients))

	// Si no tiene DB, terminamos aquí
	if !tda.HasDB() {
		fmt.Println("   ℹ️ Tenant sin DB configurada. Saltando tests de SQL.")
		return
	}

	// Probar conexión DB
	err = tda.RequireDB()
	if err != nil {
		fatal("Error de conexión a DB:", err)
	}

	// Intentar obtener usuario inexistente para probar SQL
	fmt.Println("   🔍 Probando query SQL (GetByEmail)...")
	_, _, err = tda.Users().GetByEmail(ctx, slug, "smoke-test-non-existent@example.com")
	if err == nil {
		fmt.Println("   ⚠️ Extraño: se encontró usuario que no debería existir")
	} else {
		fmt.Printf("   ✅ Respuesta DB correcta (esperado error not found): %v\n", err)
	}

	// Probar Cache
	fmt.Println("   🧠 Probando Cache...")
	err = tda.Cache().Set(ctx, "smoke-key", "smoke-value", 10*time.Second)
	if err != nil {
		fmt.Printf("   ⚠️ Error escribiendo cache: %v\n", err)
	} else {
		val, _ := tda.Cache().Get(ctx, "smoke-key")
		if val == "smoke-value" {
			fmt.Println("   ✅ Cache funciona correctamente")
		} else {
			fmt.Printf("   ⚠️ Cache devolvió valor incorrecto: %s\n", val)
		}
	}
}

func fatal(msg string, err error) {
	fmt.Printf("❌ %s %v\n", msg, err)
	os.Exit(1)
}
