package jwt

import (
	"context"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

// HybridSigningKeyStore compone un store global (DB) con un store por-tenant (FS).
// - Las operaciones globales de signingKeyStore se delegan al store global
// - Las operaciones por-tenant se delegan al FileSigningKeyStore
type HybridSigningKeyStore struct {
	global signingKeyStore
	tenant *FileSigningKeyStore
}

func NewHybridSigningKeyStore(global signingKeyStore, tenant *FileSigningKeyStore) *HybridSigningKeyStore {
	return &HybridSigningKeyStore{global: global, tenant: tenant}
}

// signingKeyStore (global)
func (h *HybridSigningKeyStore) GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error) {
	return h.global.GetActiveSigningKey(ctx)
}

func (h *HybridSigningKeyStore) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	return h.global.ListPublicSigningKeys(ctx)
}

func (h *HybridSigningKeyStore) InsertSigningKey(ctx context.Context, k *core.SigningKey) error {
	return h.global.InsertSigningKey(ctx, k)
}

// tenantSigningKeyStore (per-tenant) via FS
func (h *HybridSigningKeyStore) GetActiveSigningKeyForTenant(ctx context.Context, tenant string) (*core.SigningKey, error) {
	return h.tenant.GetActiveSigningKeyForTenant(ctx, tenant)
}

func (h *HybridSigningKeyStore) ListPublicSigningKeysForTenant(ctx context.Context, tenant string) ([]core.SigningKey, error) {
	return h.tenant.ListPublicSigningKeysForTenant(ctx, tenant)
}

// RotateFor implementa la rotación por tenant delegando al store FS
func (h *HybridSigningKeyStore) RotateFor(tenant string, graceSeconds int64) (*core.SigningKey, error) {
	return h.tenant.RotateFor(tenant, graceSeconds)
}
