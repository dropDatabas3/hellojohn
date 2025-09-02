package jwt

import (
	"crypto/ed25519"
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ParseEdDSA valida firma (EdDSA) usando el keystore (por kid o activa),
// chequea iss (si expectedIss != ""), y valida exp/nbf con una pequeña tolerancia.
// Devuelve las claims como map[string]any.
func ParseEdDSA(token string, ks *PersistentKeystore, expectedIss string) (map[string]any, error) {
	keyfunc := func(t *jwtv5.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid != "" {
			return ks.PublicKeyByKID(kid)
		}
		_, _, pub, err := ks.Active()
		if err != nil {
			return nil, err
		}
		return ed25519.PublicKey(pub), nil
	}

	tok, err := jwtv5.Parse(token, keyfunc, jwtv5.WithValidMethods([]string{"EdDSA"}))
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid_jwt")
	}

	claims, ok := tok.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, errors.New("claims_type")
	}

	// iss check (opcional)
	if expectedIss != "" {
		if iss, _ := claims["iss"].(string); iss != expectedIss {
			return nil, ErrInvalidIssuer
		}
	}

	now := time.Now()
	// exp
	if expf, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(expf), 0).Before(now.Add(-30 * time.Second)) {
			return nil, errors.New("expired")
		}
	}
	// nbf
	if nbff, ok := claims["nbf"].(float64); ok {
		if time.Unix(int64(nbff), 0).After(now.Add(30 * time.Second)) {
			return nil, errors.New("not_before")
		}
	}

	out := make(map[string]any, len(claims))
	for k, v := range claims {
		out[k] = v
	}
	return out, nil
}
