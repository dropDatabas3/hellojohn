package jwt

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/types"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TenantResolver es una función para mapear tenant ID (UUID) a slug.
// Se inyecta opcionalmente para resolver "tid" claims a slugs.
type TenantResolver func(ctx context.Context, tenantID string) (slug string, err error)

// Issuer firma tokens usando la clave activa del keystore persistente.
type Issuer struct {
	Iss            string              // "iss" base
	Keys           *PersistentKeystore // keystore persistente
	AccessTTL      time.Duration       // TTL por defecto de Access/ID (ej: 15m)
	TenantResolver TenantResolver      // opcional: para mapear tid→slug
}

func NewIssuer(iss string, ks *PersistentKeystore) *Issuer {
	return &Issuer{
		Iss:       iss,
		Keys:      ks,
		AccessTTL: 15 * time.Minute,
	}
}

// WithTenantResolver agrega un resolver de tenants.
func (i *Issuer) WithTenantResolver(resolver TenantResolver) *Issuer {
	i.TenantResolver = resolver
	return i
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
// y un issuer efectivo resuelto por configuración.
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

// IssueIDTokenForTenant emite un ID Token OIDC para un tenant específico.
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
	ErrInvalidIssuer   = errors.New("invalid_issuer")
	ErrInvalidAudience = errors.New("invalid_audience")
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
// mode acepta string para compatibilidad con controlplane/v1.IssuerMode
func ResolveIssuer(baseURL string, mode string, tenantSlug, override string) string {
	if override != "" {
		return strings.TrimRight(override, "/")
	}
	base := strings.TrimRight(baseURL, "/")
	switch types.IssuerMode(mode) {
	case types.IssuerModePath:
		return fmt.Sprintf("%s/t/%s", base, tenantSlug)
	case types.IssuerModeDomain:
		// futuro: slug subdominio (requiere DNS)
		return fmt.Sprintf("%s/t/%s", base, tenantSlug) // por ahora
	default:
		return base // global
	}
}

// KeyfuncFromTokenClaims intenta derivar el tenant a partir de los claims (tid) o del iss (modo path /t/{slug})
// y usa PublicKeyByKIDForTenant para validar la firma.
func (i *Issuer) KeyfuncFromTokenClaims() jwtv5.Keyfunc {
	return func(t *jwtv5.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("kid_missing")
		}

		var tenantSlug string
		if mc, ok := t.Claims.(jwtv5.MapClaims); ok {
			// 1) Intentar desde iss: .../t/{slug}
			if issRaw, okIss := mc["iss"].(string); okIss && issRaw != "" {
				if u, err := url.Parse(issRaw); err == nil {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					for idx := 0; idx < len(parts)-1; idx++ {
						if parts[idx] == "t" && idx+1 < len(parts) {
							tenantSlug = parts[idx+1]
						}
					}
				}
			}

			// 2) Si no se obtuvo desde iss, usar tid. Si parece UUID, mapear a slug vía resolver.
			if tenantSlug == "" {
				if v, okTid := mc["tid"].(string); okTid && v != "" {
					if _, err := uuid.Parse(v); err == nil {
						// tid es UUID; intentar mapear a slug a través del resolver
						if i.TenantResolver != nil {
							if slug, err := i.TenantResolver(context.Background(), v); err == nil {
								tenantSlug = slug
							}
						}
					} else {
						// tid parece un slug
						tenantSlug = v
					}
				}
			}
		}

		// 3) Intentar buscar en el Tenant Keyring
		if tenantSlug != "" {
			pub, err := i.Keys.PublicKeyByKIDForTenant(tenantSlug, kid)
			if err == nil {
				return ed25519.PublicKey(pub), nil
			}
			// Si falla, continuar con fallback global
		}

		// 4) Fallback: Buscar en el Global Keyring
		pub, err := i.Keys.PublicKeyByKID(kid)
		if err != nil {
			return nil, fmt.Errorf("kid_not_found: %s (tenant=%s)", kid, tenantSlug)
		}
		return ed25519.PublicKey(pub), nil
	}
}

// IssueAdminAccess emite un Access Token para un administrador.
// El token incluye claims específicos de admin: admin_type y tenants asignados.
// Audience siempre es "hellojohn:admin".
func (i *Issuer) IssueAdminAccess(ctx context.Context, claims AdminAccessClaims) (string, int, error) {
	now := time.Now().UTC()
	exp := now.Add(i.AccessTTL)

	kid, priv, _, err := i.Keys.Active()
	if err != nil {
		return "", 0, err
	}

	jwtClaims := jwtv5.MapClaims{
		"iss":        i.Iss,
		"sub":        claims.AdminID,
		"aud":        "hellojohn:admin",
		"email":      claims.Email,
		"admin_type": claims.AdminType,
		"iat":        now.Unix(),
		"nbf":        now.Unix(),
		"exp":        exp.Unix(),
	}

	// Solo incluir tenants si no está vacío (tenant admin)
	if len(claims.Tenants) > 0 {
		jwtClaims["tenants"] = claims.Tenants
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, jwtClaims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"

	signed, err := tk.SignedString(priv)
	if err != nil {
		return "", 0, err
	}

	expiresIn := int(i.AccessTTL.Seconds())
	return signed, expiresIn, nil
}

// VerifyAdminAccess verifica un admin access token y retorna los claims.
func (i *Issuer) VerifyAdminAccess(ctx context.Context, token string) (*AdminAccessClaims, error) {
	// Parse y validar firma usando el keystore
	rawClaims, err := ParseEdDSA(token, i.Keys, i.Iss)
	if err != nil {
		return nil, err
	}

	// Verificar audience
	if aud, ok := rawClaims["aud"].(string); !ok || aud != "hellojohn:admin" {
		return nil, ErrInvalidAudience
	}

	// Extraer claims específicos de admin
	adminID, _ := rawClaims["sub"].(string)
	email, _ := rawClaims["email"].(string)
	adminType, _ := rawClaims["admin_type"].(string)

	if adminID == "" || email == "" || adminType == "" {
		return nil, errors.New("missing required admin claims")
	}

	claims := &AdminAccessClaims{
		AdminID:   adminID,
		Email:     email,
		AdminType: adminType,
	}

	// Extraer tenants si existen (opcional para global admins)
	if tenantsRaw, ok := rawClaims["tenants"]; ok {
		switch v := tenantsRaw.(type) {
		case []string:
			claims.Tenants = v
		case []interface{}:
			for _, t := range v {
				if s, ok := t.(string); ok {
					claims.Tenants = append(claims.Tenants, s)
				}
			}
		}
	}

	return claims, nil
}
