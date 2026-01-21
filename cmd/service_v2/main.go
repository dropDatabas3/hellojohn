package main

import (
	"log"
	"net/http"
	"os"
	"time"

	v2server "github.com/dropDatabas3/hellojohn/internal/http/v2/server"

	// CRITICAL: Import adapters to register them via init()
	_ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/dal"
)

func main() {
	v2Addr := os.Getenv("V2_SERVER_ADDR")
	if v2Addr == "" {
		v2Addr = ":8082" // Default separate port for V2 testing
	}

	log.Printf("Starting V2 Server on %s", v2Addr)

	v2h, v2cleanup, err := v2server.BuildV2Handler()
	if err != nil {
		log.Fatalf("v2 wiring failed: %v", err)
	}
	defer func() {
		if err := v2cleanup(); err != nil {
			log.Printf("v2 cleanup error: %v", err)
		}
	}()

	srv := &http.Server{
		Addr:         v2Addr,
		Handler:      v2h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("v2 server failed: %v", err)
	}
}
