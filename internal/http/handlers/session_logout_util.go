package handlers

import (
	"crypto/sha256"
	"encoding/base64"
)

func tokensSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
