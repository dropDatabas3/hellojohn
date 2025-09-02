package jwt

import (
	"crypto/ed25519"
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Issuer firma tokens usando la clave activa del keystore persistente.
type Issuer struct {
	Iss       string              // "iss"
	Keys      *PersistentKeystore // keystore persistente
	AccessTTL time.Duration       // TTL por defecto de Access/ID (ej: 15m)
}

func NewIssuer(iss string, ks *PersistentKeystore) *Issuer {
	return &Issuer{
		Iss:       iss,
		Keys:      ks,
		AccessTTL: 15 * time.Minute,
	}
}

// ActiveKID devuelve el KID activo actual.
func (i *Issuer) ActiveKID() (string, error) {
	kid, _, _, err := i.Keys.Active()
	return kid, err
}

// Keyfunc devuelve un jwt.Keyfunc que elige la pubkey por 'kid' del token (active/retiring).
func (i *Issuer) Keyfunc() jwtv5.Keyfunc {
	return func(t *jwtv5.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid != "" {
			return i.Keys.PublicKeyByKID(kid)
		}
		// Fallback: usar la activa
		_, _, pub, err := i.Keys.Active()
		if err != nil {
			return nil, err
		}
		return ed25519.PublicKey(pub), nil
	}
}

// SignRaw firma un MapClaims arbitrario, setea header kid/typ y devuelve el JWT firmado.
func (i *Issuer) SignRaw(claims jwtv5.MapClaims) (string, string, error) {
	kid, priv, _, err := i.Keys.Active()
	if err != nil {
		return "", "", err
	}
	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"
	signed, err := tk.SignedString(priv)
	if err != nil {
		return "", "", err
	}
	return signed, kid, nil
}

// IssueAccess emite un Access Token con claims estándar + std (flat) y custom (anidado).
func (i *Issuer) IssueAccess(sub, aud string, std map[string]any, custom map[string]any) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	kid, priv, _, err := i.Keys.Active()
	if err != nil {
		return "", time.Time{}, err
	}

	claims := jwtv5.MapClaims{
		"iss": i.Iss,
		"sub": sub,
		"aud": aud,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		claims[k] = v
	}
	if custom != nil {
		claims["custom"] = custom
	}
	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"

	signed, err := tk.SignedString(priv)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// IssueIDToken emite un ID Token OIDC con claims estándar y extras.
func (i *Issuer) IssueIDToken(sub, aud string, std map[string]any, extra map[string]any) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	kid, priv, _, err := i.Keys.Active()
	if err != nil {
		return "", time.Time{}, err
	}

	claims := jwtv5.MapClaims{
		"iss": i.Iss,
		"sub": sub,
		"aud": aud,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		claims[k] = v
	}
	if extra != nil {
		for k, v := range extra {
			claims[k] = v
		}
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"

	signed, err := tk.SignedString(priv)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// JWKSJSON expone el JWKS actual (active+retiring)
func (i *Issuer) JWKSJSON() []byte {
	j, _ := i.Keys.JWKSJSON()
	return j
}

// Helpers defensivos para errores comunes
var (
	ErrInvalidIssuer = errors.New("invalid_issuer")
)

// ───────────────────────────────────────────────────────────────
// helpers para firmar claims arbitrarios EdDSA
// ───────────────────────────────────────────────────────────────

// SignEdDSA firma claims arbitrarios con la clave activa (no inyecta iss/exp/iat).
// Útil para firmar "state" de flows sociales, etc.
func (i *Issuer) SignEdDSA(claims map[string]any) (string, error) {
	mc := jwtv5.MapClaims{}
	for k, v := range claims {
		mc[k] = v
	}
	signed, _, err := i.SignRaw(mc)
	return signed, err
}
