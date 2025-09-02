package jwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
)

// GenerateEd25519 genera un par ed25519.
func GenerateEd25519() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// EncodeBase64URL devuelve base64url sin padding.
func EncodeBase64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
