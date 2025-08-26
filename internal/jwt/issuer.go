package jwt

import (
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type Issuer struct {
	Iss       string // "iss" claim (ej: URL o URN de tu servicio)
	Keys      *KeySet
	AccessTTL time.Duration // default 15m
}

func NewIssuer(iss string, keys *KeySet) *Issuer {
	return &Issuer{
		Iss:       iss,
		Keys:      keys,
		AccessTTL: 15 * time.Minute,
	}
}

// IssueAccess emite un JWT firmado EdDSA con claims std + opcionales.
// std puede incluir "tid", "amr", etc. custom se puede meter en "custom".
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
	// std flat
	for k, v := range std {
		claims[k] = v
	}
	// custom nested para no pisar std
	if custom != nil {
		claims["custom"] = custom
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = i.Keys.KID

	signed, err := tk.SignedString(i.Keys.Priv)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}
