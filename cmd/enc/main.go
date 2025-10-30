package main

import (
	"fmt"
	"log"
	"os"

	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")
	key := os.Getenv("SECRETBOX_MASTER_KEY")
	if key == "" {
		log.Fatal("SECRETBOX_MASTER_KEY not set")
	}
	dsn := os.Getenv("STORAGE_DSN")
	if dsn == "" {
		log.Fatal("STORAGE_DSN not set")
	}
	enc, err := sec.Encrypt(dsn)
	if err != nil {
		log.Fatalf("encrypt: %v", err)
	}
	fmt.Println(enc)
}
