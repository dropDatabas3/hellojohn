package jwt

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
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

// KeyfuncForTenant devuelve un jwt.Keyfunc tenant-aware que resuelve la pubkey por KID
// dentro del JWKS del tenant. Si no encuentra el KID, retorna error y el token falla.
func (i *Issuer) KeyfuncForTenant(tenant string) jwtv5.Keyfunc {
	return func(t *jwtv5.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("kid_missing")
		}
		// Buscar pubkey en JWKS del tenant
		pub, err := i.Keys.PublicKeyByKIDForTenant(tenant, kid)
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
	for k, v := range extra {
		claims[k] = v
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

// IssueAccessForTenant emite un Access Token para un tenant específico usando su clave activa
// y un issuer efectivo resuelto por configuración (path/domain/override).
// - tenant: slug del tenant (para keystore multi-tenant)
// - iss: issuer efectivo a inyectar en el claim "iss"
func (i *Issuer) IssueAccessForTenant(tenant, iss, sub, aud string, std map[string]any, custom map[string]any) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	kid, priv, _, err := i.Keys.ActiveForTenant(tenant)
	if err != nil {
		return "", time.Time{}, err
	}

	claims := jwtv5.MapClaims{
		"iss": iss,
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

// IssueIDTokenForTenant emite un ID Token OIDC para un tenant específico usando su clave activa
// y un issuer efectivo provisto.
func (i *Issuer) IssueIDTokenForTenant(tenant, iss, sub, aud string, std map[string]any, extra map[string]any) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	kid, priv, _, err := i.Keys.ActiveForTenant(tenant)
	if err != nil {
		return "", time.Time{}, err
	}

	claims := jwtv5.MapClaims{
		"iss": iss,
		"sub": sub,
		"aud": aud,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		claims[k] = v
	}
	for k, v := range extra {
		claims[k] = v
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

// ResolveIssuer construye el issuer efectivo por tenant según settings del control-plane.
// - Si override no está vacío, lo usa tal cual (sin trailing slash)
// - Path:   {base}/t/{slug}
// - Domain: futuro (por ahora igual que Path)
// - Global: base
func ResolveIssuer(baseURL string, mode controlplane.IssuerMode, tenantSlug, override string) string {
	if override != "" {
		return strings.TrimRight(override, "/")
	}
	base := strings.TrimRight(baseURL, "/")
	switch mode {
	case controlplane.IssuerModePath:
		return fmt.Sprintf("%s/t/%s", base, tenantSlug)
	case controlplane.IssuerModeDomain:
		// futuro: slug subdominio (requiere DNS)
		// return fmt.Sprintf("https://%s.%s", tenantSlug, strings.TrimPrefix(base, "https://"))
		return fmt.Sprintf("%s/t/%s", base, tenantSlug) // por ahora
	default:
		return base // global
	}
}

// KeyfuncFromTokenClaims intenta derivar el tenant a partir de los claims (tid) o del iss (modo path /t/{slug})
// y usa PublicKeyByKIDForTenant para validar la firma. Retorna error si no logra derivar tenant o no encuentra KID.
// Nota: requiere que el parser llene MapClaims.
func (i *Issuer) KeyfuncFromTokenClaims() jwtv5.Keyfunc {
	return func(t *jwtv5.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("kid_missing")
		}
		// Intentar leer tid
		var tenant string
		if mc, ok := t.Claims.(jwtv5.MapClaims); ok {
			if v, ok2 := mc["tid"].(string); ok2 && v != "" {
				tenant = v
			} else if issRaw, ok3 := mc["iss"].(string); ok3 && issRaw != "" {
				// Si el issuer es path-mode .../t/{slug}, intentar extraer el slug final
				if u, err := url.Parse(issRaw); err == nil {
					// ruta como /.../t/{slug}
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					// Buscar patrón "t/{slug}" al final
					for idx := 0; idx < len(parts)-1; idx++ {
						if parts[idx] == "t" && idx+1 < len(parts) {
							tenant = parts[idx+1]
						}
					}
				}
				if tenant == "" {
					// Fallback: última parte del path
					segs := strings.Split(strings.Trim(issRaw, "/"), "/")
					if len(segs) > 0 {
						tenant = segs[len(segs)-1]
					}
				}
			}
		}
		if tenant == "" {
			return nil, errors.New("tenant_unresolved")
		}
		pub, err := i.Keys.PublicKeyByKIDForTenant(tenant, kid)
		if err != nil {
			return nil, err
		}
		return ed25519.PublicKey(pub), nil
	}
}
