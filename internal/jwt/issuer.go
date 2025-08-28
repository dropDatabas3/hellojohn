package jwt

import (
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Issuer firma tokens JWT (EdDSA) y define TTLs.
type Issuer struct {
	Iss       string        // "iss" claim (URL base del servicio)
	Keys      *KeySet       // Claves Ed25519 (Priv, Pub, KID)
	AccessTTL time.Duration // TTL por defecto (ej. 15m) aplicado también a ID Tokens
}

// NewIssuer crea un emisor con TTL por defecto 15m.
func NewIssuer(iss string, keys *KeySet) *Issuer {
	return &Issuer{
		Iss:       iss,
		Keys:      keys,
		AccessTTL: 15 * time.Minute,
	}
}

// IssueAccess emite un Access Token con claims estándar + std (flat) y custom (anidado).
// std puede incluir "tid", "amr", "scp", etc. custom va dentro de "custom".
func (i *Issuer) IssueAccess(sub, aud string, std map[string]any, custom map[string]any) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	claims := jwtv5.MapClaims{
		"iss": i.Iss,
		"sub": sub,
		"aud": aud,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	// std (flat)
	for k, v := range std {
		claims[k] = v
	}
	// custom (anidado)
	if custom != nil {
		claims["custom"] = custom
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = i.Keys.KID
	tk.Header["typ"] = "JWT"

	signed, err := tk.SignedString(i.Keys.Priv)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// IssueIDToken emite un ID Token OIDC con claims estándar y extras en top-level.
// Usar std para todo lo que quieras garantizar en top-level (p.ej. tid, at_hash).
// En extra podés pasar "nonce", "auth_time", "acr", "amr" u otros benignos.
func (i *Issuer) IssueIDToken(sub, aud string, std map[string]any, extra map[string]any) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	claims := jwtv5.MapClaims{
		"iss": i.Iss,
		"sub": sub,
		"aud": aud,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}

	// std (flat, prioridad para asegurar presencia en top-level)
	for k, v := range std {
		claims[k] = v
	}

	// extras (flat; no los anidamos para OIDC)
	if extra != nil {
		for k, v := range extra {
			claims[k] = v
		}
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = i.Keys.KID
	tk.Header["typ"] = "JWT"

	signed, err := tk.SignedString(i.Keys.Priv)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}
