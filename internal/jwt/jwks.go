package jwt

import (
	"encoding/json"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type jwk struct {
	Kty string `json:"kty"` // "OKP"
	Crv string `json:"crv"` // "Ed25519"
	Kid string `json:"kid"`
	Alg string `json:"alg"` // "EdDSA"
	Use string `json:"use"` // "sig"
	X   string `json:"x"`   // base64url(pub)
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

func buildJWKS(keys []core.SigningKey) []byte {
	out := jwks{Keys: make([]jwk, 0, len(keys))}
	for _, k := range keys {
		out.Keys = append(out.Keys, jwk{
			Kty: "OKP",
			Crv: "Ed25519",
			Kid: k.KID,
			Alg: k.Alg,
			Use: "sig",
			X:   EncodeBase64URL(k.PublicKey),
		})
	}
	b, _ := json.Marshal(out)
	return b
}
