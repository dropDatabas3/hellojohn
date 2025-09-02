package jwt

import (
	"context"
	"crypto/ed25519"
	"errors"
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
	k.activePriv = ed25519.PrivateKey(rec.PrivateKey)
	k.activePub = ed25519.PublicKey(rec.PublicKey)
	k.cacheUntil = time.Now().Add(k.cacheTTL)
	return k.activeKID, k.activePriv, k.activePub, nil
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
