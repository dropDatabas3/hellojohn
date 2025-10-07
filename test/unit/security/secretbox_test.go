package security

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	// Remover t.Parallel() por conflicto con UnsafeReset global
	secretbox.UnsafeResetSecretBoxForTests()

	// Clave de 32 bytes -> base64
	raw := make([]byte, 32)
	for i := 0; i < 32; i++ {
		raw[i] = byte(i + 1)
	}
	os.Setenv("SECRETBOX_MASTER_KEY", base64.StdEncoding.EncodeToString(raw))
	defer os.Unsetenv("SECRETBOX_MASTER_KEY")

	msg := "hola mundo ✓ — secreto"
	ct, err := secretbox.Encrypt(msg)
	if err != nil {
		t.Fatalf("Encrypt err: %v", err)
	}
	pt, err := secretbox.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt err: %v", err)
	}
	if pt != msg {
		t.Fatalf("plaintext mismatch: got %q want %q", pt, msg)
	}
}

func TestDecrypt_DetectsTamper(t *testing.T) {
	// Remover t.Parallel() por conflicto con UnsafeReset global
	secretbox.UnsafeResetSecretBoxForTests()

	raw := make([]byte, 32)
	for i := 0; i < 32; i++ {
		raw[i] = byte(255 - i)
	}
	os.Setenv("SECRETBOX_MASTER_KEY", base64.StdEncoding.EncodeToString(raw))
	defer os.Unsetenv("SECRETBOX_MASTER_KEY")

	ct, err := secretbox.Encrypt("top secret")
	if err != nil {
		t.Fatalf("Encrypt err: %v", err)
	}
	parts := strings.Split(ct, "|")
	if len(parts) != 2 {
		t.Fatalf("unexpected ct format")
	}
	// corromper un byte del ciphertext (base64 -> bytes -> flip -> base64)
	bs, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) == 0 {
		t.Fatal("empty ct")
	}
	bs[0] ^= 0x01 // flip
	parts[1] = base64.StdEncoding.EncodeToString(bs)
	corrupted := parts[0] + "|" + parts[1]

	if _, err := secretbox.Decrypt(corrupted); err == nil {
		t.Fatalf("expected auth error, got nil")
	}
}

func TestEncrypt_ErrorWhenNoKey(t *testing.T) {
	// Remover t.Parallel() por conflicto con UnsafeReset global
	secretbox.UnsafeResetSecretBoxForTests()
	os.Unsetenv("SECRETBOX_MASTER_KEY")

	if _, err := secretbox.Encrypt("x"); err == nil {
		t.Fatalf("expected error when key missing")
	}
}
