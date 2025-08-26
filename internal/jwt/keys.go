package jwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
)

// KeySet mantiene una sola clave activa (dev). Luego metemos rotación.
type KeySet struct {
	Priv ed25519.PrivateKey
	Pub  ed25519.PublicKey
	KID  string
	Alg  string // "EdDSA"
}

// NewDevEd25519 genera una clave Ed25519 en memoria con un KID dado.
func NewDevEd25519(kid string) (*KeySet, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeySet{
		Priv: priv,
		Pub:  pub,
		KID:  kid,
		Alg:  "EdDSA",
	}, nil
}

// ----- JWKS (serialización) -----

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

// JWKSJSON devuelve el JWKS (solo la pública) en JSON.
func (k *KeySet) JWKSJSON() []byte {
	j := jwks{
		Keys: []jwk{{
			Kty: "OKP",
			Crv: "Ed25519",
			Kid: k.KID,
			Alg: k.Alg,
			Use: "sig",
			X:   base64.RawURLEncoding.EncodeToString(k.Pub),
		}},
	}
	b, _ := json.Marshal(j)
	return b
}
