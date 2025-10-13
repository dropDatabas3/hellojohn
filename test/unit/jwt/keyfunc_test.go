package jwt_test

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// fakeTenantStore is a minimal in-memory store implementing the parts of the
// keystore interfaces we need for tests. It exposes per-tenant public keys.
type fakeTenantStore struct {
	perTenant map[string][]core.SigningKey
}

func (f *fakeTenantStore) GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error) {
	return nil, core.ErrNotFound
}
func (f *fakeTenantStore) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	return nil, nil
}
func (f *fakeTenantStore) InsertSigningKey(ctx context.Context, k *core.SigningKey) error { return nil }

// tenant-aware methods
func (f *fakeTenantStore) GetActiveSigningKeyForTenant(ctx context.Context, tenant string) (*core.SigningKey, error) {
	if list, ok := f.perTenant[tenant]; ok && len(list) > 0 {
		k := list[0]
		return &k, nil
	}
	return nil, core.ErrNotFound
}
func (f *fakeTenantStore) ListPublicSigningKeysForTenant(ctx context.Context, tenant string) ([]core.SigningKey, error) {
	if list, ok := f.perTenant[tenant]; ok {
		return list, nil
	}
	return nil, core.ErrNotFound
}

func TestKeyfuncFromTokenClaims_IssuerFirst_SlugPreferred(t *testing.T) {
	// Generate a keypair for tenant "acme"
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	kid := "kid-acme-1"

	// Build fake store exposing the public key under tenant "acme"
	store := &fakeTenantStore{perTenant: map[string][]core.SigningKey{
		"acme": {
			{
				KID:       kid,
				Alg:       "EdDSA",
				PublicKey: pub,
				Status:    core.KeyActive,
				NotBefore: time.Now().Add(-time.Minute),
			},
		},
	}}
	ks := jwtx.NewPersistentKeystore(context.Background(), store)
	issuer := jwtx.NewIssuer("http://base.example", ks)

	// Build a JWT whose iss encodes the tenant slug in path (/t/acme)
	claims := jwtv5.MapClaims{
		"iss": "http://base.example/t/acme",
		"sub": "user-1",
		"aud": "web-frontend",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Add(-10 * time.Second).Unix(),
		// Provide a misleading tid on purpose; keyfunc should prefer iss-derived slug
		"tid": "not-the-slug",
	}
	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"
	signed, err := tk.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	parsed, err := jwtv5.Parse(signed, issuer.KeyfuncFromTokenClaims(), jwtv5.WithValidMethods([]string{"EdDSA"}))
	if err != nil || !parsed.Valid {
		t.Fatalf("expected valid token, got err=%v valid=%v", err, parsed.Valid)
	}

	// Negative: change iss to a different slug; resolution should fail (kid not found under that tenant)
	claims["iss"] = "http://base.example/t/beta"
	tk2 := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk2.Header["kid"] = kid
	tok2, err := tk2.SignedString(priv)
	if err != nil {
		t.Fatalf("sign2: %v", err)
	}
	if parsed2, err2 := jwtv5.Parse(tok2, issuer.KeyfuncFromTokenClaims(), jwtv5.WithValidMethods([]string{"EdDSA"})); err2 == nil && parsed2.Valid {
		t.Fatalf("expected parse to fail for mismatched tenant slug")
	}
}
