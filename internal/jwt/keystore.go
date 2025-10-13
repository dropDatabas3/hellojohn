package jwt

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

var (
	ErrNoActiveKey = errors.New("no_active_signing_key")
)

type signingKeyStore interface {
	GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error)
	ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error)
	InsertSigningKey(ctx context.Context, k *core.SigningKey) error
}

// tenantSigningKeyStore extiende la store para escenarios multi-tenant opcionales.
// No es obligatorio implementarla; si no está disponible, se usa el fallback global.
type tenantSigningKeyStore interface {
	GetActiveSigningKeyForTenant(ctx context.Context, tenant string) (*core.SigningKey, error)
	ListPublicSigningKeysForTenant(ctx context.Context, tenant string) ([]core.SigningKey, error)
}

// tenantKeyRotator expone rotación de claves por tenant (FS store o híbrido)
type tenantKeyRotator interface {
	RotateFor(tenant string, graceSeconds int64) (*core.SigningKey, error)
}

// PersistentKeystore mantiene cache local y lee de DB.
type PersistentKeystore struct {
	ctx   context.Context
	store signingKeyStore

	mu         sync.RWMutex
	activeKID  string
	activePriv ed25519.PrivateKey
	activePub  ed25519.PublicKey
	cacheUntil time.Time
	cacheTTL   time.Duration

	lastJWKS  []byte
	jwksUntil time.Time
	jwksTTL   time.Duration
}

func NewPersistentKeystore(ctx context.Context, s signingKeyStore) *PersistentKeystore {
	return &PersistentKeystore{
		ctx:      ctx,
		store:    s,
		cacheTTL: 30 * time.Second,
		jwksTTL:  15 * time.Second,
	}
}

// EnsureBootstrap: si no hay clave activa, genera una.
func (k *PersistentKeystore) EnsureBootstrap() error {
	_, err := k.store.GetActiveSigningKey(k.ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, core.ErrNotFound) {
		return err
	}
	pub, priv, err := GenerateEd25519()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	key := &core.SigningKey{
		KID:        "boot-" + now.Format("20060102T150405Z"),
		Alg:        "EdDSA",
		PublicKey:  pub,
		PrivateKey: priv,
		Status:     core.KeyActive,
		NotBefore:  now,
	}
	return k.store.InsertSigningKey(k.ctx, key)
}

// Active devuelve la clave activa (cacheada).
func (k *PersistentKeystore) Active() (kid string, priv ed25519.PrivateKey, pub ed25519.PublicKey, err error) {
	k.mu.RLock()
	if time.Now().Before(k.cacheUntil) && k.activeKID != "" && len(k.activePriv) > 0 {
		defer k.mu.RUnlock()
		return k.activeKID, k.activePriv, k.activePub, nil
	}
	k.mu.RUnlock()

	k.mu.Lock()
	defer k.mu.Unlock()
	if time.Now().Before(k.cacheUntil) && k.activeKID != "" && len(k.activePriv) > 0 {
		return k.activeKID, k.activePriv, k.activePub, nil
	}

	rec, err := k.store.GetActiveSigningKey(k.ctx)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return "", nil, nil, ErrNoActiveKey
		}
		return "", nil, nil, err
	}
	k.activeKID = rec.KID

	// Descifrar clave privada si está cifrada (detectar por magic header GCMV1)
	privKey := rec.PrivateKey
	if masterKey := os.Getenv("SIGNING_MASTER_KEY"); masterKey != "" && bytes.HasPrefix(rec.PrivateKey, []byte("GCMV1")) {
		// Clave cifrada detectada por magic header, descifrar
		decrypted, err := DecryptPrivateKey(rec.PrivateKey, masterKey)
		if err != nil {
			return "", nil, nil, fmt.Errorf("decrypt private key: %w", err)
		}
		privKey = decrypted
	}

	k.activePriv = ed25519.PrivateKey(privKey)
	k.activePub = ed25519.PublicKey(rec.PublicKey)
	k.cacheUntil = time.Now().Add(k.cacheTTL)
	return k.activeKID, k.activePriv, k.activePub, nil
}

// ActiveForTenant devuelve la clave activa para un tenant específico si la store lo soporta.
// Si no, retorna la clave activa global como fallback.
func (k *PersistentKeystore) ActiveForTenant(tenant string) (kid string, priv ed25519.PrivateKey, pub ed25519.PublicKey, err error) {
	if ts, ok := k.store.(tenantSigningKeyStore); ok {
		rec, e := ts.GetActiveSigningKeyForTenant(k.ctx, tenant)
		if e == nil && rec != nil {
			// Nota: no cacheamos por tenant aquí para mantenerlo simple; loaders externos pueden cachear por tenant
			// Descifrar clave privada si está cifrada
			privKey := rec.PrivateKey
			if masterKey := os.Getenv("SIGNING_MASTER_KEY"); masterKey != "" && bytes.HasPrefix(rec.PrivateKey, []byte("GCMV1")) {
				dec, derr := DecryptPrivateKey(rec.PrivateKey, masterKey)
				if derr != nil {
					return "", nil, nil, fmt.Errorf("decrypt private key: %w", derr)
				}
				privKey = dec
			}
			return rec.KID, ed25519.PrivateKey(privKey), ed25519.PublicKey(rec.PublicKey), nil
		}
	}
	// Fallback a global
	return k.Active()
}

// PublicKeyByKID devuelve la pubkey para un KID (active o retiring).
func (k *PersistentKeystore) PublicKeyByKID(kid string) (ed25519.PublicKey, error) {
	// Primero, si coincide con la activa cacheada
	k.mu.RLock()
	if kid != "" && kid == k.activeKID && len(k.activePub) > 0 {
		pub := make([]byte, len(k.activePub))
		copy(pub, k.activePub)
		k.mu.RUnlock()
		return ed25519.PublicKey(pub), nil
	}
	k.mu.RUnlock()

	// Listar publicables desde store (puede estar cacheado por el lado del store o en JWKS)
	recs, err := k.store.ListPublicSigningKeys(k.ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r.KID == kid {
			return ed25519.PublicKey(r.PublicKey), nil
		}
	}
	return nil, errors.New("kid_not_found")
}

// PublicKeyByKIDForTenant devuelve la pubkey para un KID dentro del ámbito de un tenant específico.
// Si la store es tenant-aware, busca en las claves públicas del tenant (active/retiring) y devuelve error si no hay match.
// Si la store no es tenant-aware, cae en PublicKeyByKID (global).
func (k *PersistentKeystore) PublicKeyByKIDForTenant(tenant, kid string) (ed25519.PublicKey, error) {
	if kid == "" {
		return nil, errors.New("kid_missing")
	}
	if ts, ok := k.store.(tenantSigningKeyStore); ok {
		recs, err := ts.ListPublicSigningKeysForTenant(k.ctx, tenant)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			if r.KID == kid {
				return ed25519.PublicKey(r.PublicKey), nil
			}
		}
		return nil, errors.New("kid_not_found")
	}
	// Fallback a global si no hay store tenant-aware
	return k.PublicKeyByKID(kid)
}

// JWKSJSON construye JWKS a partir de DB (cache corto).
func (k *PersistentKeystore) JWKSJSON() ([]byte, error) {
	k.mu.RLock()
	if time.Now().Before(k.jwksUntil) && len(k.lastJWKS) > 0 {
		defer k.mu.RUnlock()
		return k.lastJWKS, nil
	}
	k.mu.RUnlock()

	k.mu.Lock()
	defer k.mu.Unlock()

	if time.Now().Before(k.jwksUntil) && len(k.lastJWKS) > 0 {
		return k.lastJWKS, nil
	}

	pubKeys, err := k.store.ListPublicSigningKeys(k.ctx)
	if err != nil {
		return nil, err
	}
	j := buildJWKS(pubKeys)
	k.lastJWKS = j
	k.jwksUntil = time.Now().Add(k.jwksTTL)
	return j, nil
}

// JWKSJSONForTenant construye JWKS para un tenant específico si la store lo soporta.
// No hace fallback silencioso: si no hay claves del tenant, retorna error para permitir bootstrap externo.
func (k *PersistentKeystore) JWKSJSONForTenant(tenant string) ([]byte, error) {
	// Si la store no implementa el interfaz tenant-aware, usar global
	if ts, ok := k.store.(tenantSigningKeyStore); ok {
		recs, err := ts.ListPublicSigningKeysForTenant(k.ctx, tenant)
		if err != nil {
			return nil, err
		}
		return buildJWKS(recs), nil
	}
	return k.JWKSJSON()
}

// RotateFor rota la clave activa de un tenant y crea una nueva activa, si la store lo soporta.
func (k *PersistentKeystore) RotateFor(tenant string, graceSeconds int64) (*core.SigningKey, error) {
	if rot, ok := k.store.(tenantKeyRotator); ok {
		return rot.RotateFor(tenant, graceSeconds)
	}
	return nil, fmt.Errorf("rotation_not_supported")
}
