package helpers

import (
	"crypto/sha256"
	"encoding/base64"
)

func FromBytes(b []byte) string {
	sum := sha256.Sum256(b)
	// ETag debil, formato W/"<b64url>"
	return `W/"` + base64.RawURLEncoding.EncodeToString(sum[:]) + `"`
}
