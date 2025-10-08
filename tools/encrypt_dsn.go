package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run encrypt_dsn.go <plaintext_dsn>")
	}

	plaintext := os.Args[1]
	encrypted, err := secretbox.Encrypt(plaintext)
	if err != nil {
		log.Fatalf("Encryption failed: %v", err)
	}

	fmt.Printf("Plaintext: %s\n", plaintext)
	fmt.Printf("Encrypted: %s\n", encrypted)
}
