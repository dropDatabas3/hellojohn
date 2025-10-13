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

// fakeTenantStoreWithRetiring exposes active + retiring for a tenant to simulate grace window.
type fakeTenantStoreWithRetiring struct {
	perTenant map[string][]core.SigningKey
}

func (f *fakeTenantStoreWithRetiring) GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error) {
	return nil, core.ErrNotFound
}
func (f *fakeTenantStoreWithRetiring) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	return nil, nil
}
func (f *fakeTenantStoreWithRetiring) InsertSigningKey(ctx context.Context, k *core.SigningKey) error {
	return nil
}
func (f *fakeTenantStoreWithRetiring) GetActiveSigningKeyForTenant(ctx context.Context, tenant string) (*core.SigningKey, error) {
	if list, ok := f.perTenant[tenant]; ok && len(list) > 0 {
		k := list[0]
		return &k, nil
	}
	return nil, core.ErrNotFound
}
func (f *fakeTenantStoreWithRetiring) ListPublicSigningKeysForTenant(ctx context.Context, tenant string) ([]core.SigningKey, error) {
	if list, ok := f.perTenant[tenant]; ok {
		return list, nil
	}
	return nil, core.ErrNotFound
}

func TestIntrospection_KeyRotation_GraceAcceptsRetiring(t *testing.T) {
	// Create two keypairs: retiring and active
	pubOld, privOld, _ := ed25519.GenerateKey(nil)
	pubNew, _, _ := ed25519.GenerateKey(nil)
	now := time.Now().Add(-time.Minute)

	retiring := core.SigningKey{KID: "kid-acme-old", Alg: "EdDSA", PublicKey: pubOld, Status: core.KeyRetiring, NotBefore: now}
	active := core.SigningKey{KID: "kid-acme-new", Alg: "EdDSA", PublicKey: pubNew, Status: core.KeyActive, NotBefore: now}

	store := &fakeTenantStoreWithRetiring{perTenant: map[string][]core.SigningKey{
		"acme": {active, retiring},
	}}
	ks := jwtx.NewPersistentKeystore(context.Background(), store)
	issuer := jwtx.NewIssuer("http://base.example", ks)

	// Sign a token with the OLD private key but iss for tenant acme
	claims := jwtv5.MapClaims{
		"iss": "http://base.example/t/acme",
		"sub": "u1",
		"aud": "web-frontend",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Add(-10 * time.Second).Unix(),
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tok.Header["kid"] = retiring.KID
	tok.Header["typ"] = "JWT"
	signed, err := tok.SignedString(privOld)
	if err != nil {
		t.Fatalf("sign retiring: %v", err)
	}

	// Parsing via tenant-aware keyfunc should succeed because retiring key is still published in JWKS
	if parsed, err := jwtv5.Parse(signed, issuer.KeyfuncFromTokenClaims(), jwtv5.WithValidMethods([]string{"EdDSA"})); err != nil || !parsed.Valid {
		t.Fatalf("expected retiring key to be accepted during grace, err=%v", err)
	}
}
