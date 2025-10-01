package internal

import (
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// VerifyJWTWithJWKS verifies a JWT's signature using the issuer JWKS and asserts iss/aud/exp/nbf/iat with leeway.
// - Supports alg: RS256 and EdDSA (Ed25519)
// - expectedAud may be empty to skip aud check; same for expectedIss.
func VerifyJWTWithJWKS(ctx context.Context, baseURL, token, expectedIss, expectedAud string, leeway time.Duration) (jwt.MapClaims, map[string]any, error) {
	if token == "" {
		return nil, nil, errors.New("empty token")
	}

	// Parse header first to get alg/kid
	header, _, err := splitJWT(token)
	if err != nil {
		return nil, nil, fmt.Errorf("parse header: %w", err)
	}
	alg, _ := header["alg"].(string)
	if alg == "" {
		return nil, header, fmt.Errorf("missing alg in header")
	}
	if !equalsAnyFold(alg, "RS256", "EdDSA") {
		return nil, header, fmt.Errorf("unexpected alg %q", alg)
	}

	kid, _ := header["kid"].(string)

	// Fetch JWKS
	jwksURL := strings.TrimRight(baseURL, "/") + "/.well-known/jwks.json"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, header, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, header, fmt.Errorf("jwks status=%d", resp.StatusCode)
	}
	var jwks struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, header, fmt.Errorf("decode jwks: %w", err)
	}
	if len(jwks.Keys) == 0 {
		return nil, header, errors.New("empty JWKS")
	}

	// Resolve key by kid if present; otherwise try all compatible keys
	var candidates []jwk
	if kid != "" {
		for _, k := range jwks.Keys {
			if k.Kid == kid {
				candidates = append(candidates, k)
				break
			}
		}
		if len(candidates) == 0 {
			return nil, header, fmt.Errorf("kid %q not found in JWKS", kid)
		}
	} else {
		candidates = jwks.Keys
	}

	// Build keyfunc that iterates candidates (if no direct match by kty/alg it errors)
	keyFunc := func(t *jwt.Token) (any, error) {
		// Enforce allowed algs
		if t.Method.Alg() != alg {
			return nil, fmt.Errorf("alg mismatch: token=%s parser=%s", t.Method.Alg(), alg)
		}
		var lastErr error
		for _, k := range candidates {
			switch {
			case equalsAnyFold(alg, "RS256") && strings.EqualFold(k.Kty, "RSA"):
				pk, err := k.asRSAPublicKey()
				if err != nil {
					lastErr = err
					continue
				}
				return pk, nil
			case equalsAnyFold(alg, "EdDSA") && (strings.EqualFold(k.Kty, "OKP") || strings.EqualFold(k.Crv, "Ed25519")):
				pk, err := k.asEd25519PublicKey()
				if err != nil {
					lastErr = err
					continue
				}
				return pk, nil
			default:
				lastErr = fmt.Errorf("no compatible key (kty=%s crv=%s) for alg=%s", k.Kty, k.Crv, alg)
			}
		}
		if lastErr == nil {
			lastErr = errors.New("no candidate key usable")
		}
		return nil, lastErr
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, keyFunc, jwt.WithLeeway(leeway), jwt.WithValidMethods([]string{"RS256", "EdDSA"}))
	if err != nil {
		return nil, header, fmt.Errorf("parse/verify: %w", err)
	}
	if !parsed.Valid {
		return nil, header, errors.New("token invalid")
	}

	// Manual iss/aud & time checks (robustness across different claim shapes)
	now := time.Now()
	// iss
	if expectedIss != "" {
		if iss, _ := getString(claims, "iss"); iss != expectedIss {
			return nil, header, fmt.Errorf("iss mismatch: got=%q want=%q", iss, expectedIss)
		}
	}
	// aud (can be string or array)
	if expectedAud != "" {
		if !audContains(claims["aud"], expectedAud) {
			return nil, header, fmt.Errorf("aud does not contain %q (got=%v)", expectedAud, claims["aud"])
		}
	}
	// exp
	if exp, ok := getUnixTime(claims, "exp"); ok {
		if now.After(exp.Add(leeway)) {
			return nil, header, fmt.Errorf("expired: exp=%d now=%d", exp.Unix(), now.Unix())
		}
	}
	// nbf
	if nbf, ok := getUnixTime(claims, "nbf"); ok {
		if now.Add(leeway).Before(nbf) {
			return nil, header, fmt.Errorf("not before: nbf=%d now=%d", nbf.Unix(), now.Unix())
		}
	}
	// iat (must not be in the future beyond leeway)
	if iat, ok := getUnixTime(claims, "iat"); ok {
		if iat.After(now.Add(leeway)) {
			return nil, header, fmt.Errorf("issued in future: iat=%d now=%d", iat.Unix(), now.Unix())
		}
	}

	return claims, header, nil
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// OKP/Ed25519
	Crv string `json:"crv"`
	X   string `json:"x"`
}

func (j jwk) asRSAPublicKey() (*rsa.PublicKey, error) {
	if j.N == "" || j.E == "" {
		return nil, errors.New("rsa jwk missing n/e")
	}
	nBytes, err := b64urlDecode(j.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := b64urlDecode(j.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	var nb big.Int
	nb.SetBytes(nBytes)
	// e may be small; convert bytes to int
	e := 0
	for _, b := range eBytes {
		e = (e << 8) | int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid e=0")
	}
	return &rsa.PublicKey{N: &nb, E: e}, nil
}

func (j jwk) asEd25519PublicKey() (ed25519.PublicKey, error) {
	if !strings.EqualFold(j.Crv, "Ed25519") && j.Crv != "" {
		return nil, fmt.Errorf("unsupported OKP crv=%s", j.Crv)
	}
	if j.X == "" {
		return nil, errors.New("okp jwk missing x")
	}
	xBytes, err := b64urlDecode(j.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	if l := len(xBytes); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("unexpected ed25519 key length=%d", l)
	}
	return ed25519.PublicKey(xBytes), nil
}

// Utility helpers (self-contained; do not rely on other test packages)
func b64urlDecode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.StdEncoding.DecodeString(s)
}

func splitJWT(tok string) (map[string]any, map[string]any, error) {
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return nil, nil, errors.New("invalid token format")
	}
	hb, err := b64urlDecode(parts[0])
	if err != nil {
		return nil, nil, err
	}
	pb, err := b64urlDecode(parts[1])
	if err != nil {
		return nil, nil, err
	}
	var hdr, pld map[string]any
	if err := json.Unmarshal(hb, &hdr); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(pb, &pld); err != nil {
		return nil, nil, err
	}
	return hdr, pld, nil
}

func equalsAnyFold(s string, opts ...string) bool {
	for _, o := range opts {
		if strings.EqualFold(s, o) {
			return true
		}
	}
	return false
}

func getString(claims jwt.MapClaims, key string) (string, bool) {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

func getUnixTime(claims jwt.MapClaims, key string) (time.Time, bool) {
	v, ok := claims[key]
	if !ok {
		return time.Time{}, false
	}
	switch t := v.(type) {
	case float64:
		return time.Unix(int64(t), 0), true
	case json.Number:
		iv, err := t.Int64()
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(iv, 0), true
	case int64:
		return time.Unix(t, 0), true
	case int:
		return time.Unix(int64(t), 0), true
	default:
		return time.Time{}, false
	}
}

func audContains(v any, want string) bool {
	if want == "" {
		return true
	}
	switch a := v.(type) {
	case string:
		return strings.EqualFold(a, want)
	case []any:
		for _, it := range a {
			if s, _ := it.(string); strings.EqualFold(s, want) {
				return true
			}
		}
		return false
	case []string:
		for _, s := range a {
			if strings.EqualFold(s, want) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
