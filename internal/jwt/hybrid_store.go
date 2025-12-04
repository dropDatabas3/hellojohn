package jwt

import (
	"context"
	"fmt"

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
	k, err := h.global.GetActiveSigningKey(ctx)
	if err == nil {
		return k, nil
	}
	fmt.Printf("DEBUG: Hybrid global store failed: %v. Falling back to tenant store.\n", err)
	// Fallback to file store if global fails (e.g. DB down)
	res, err2 := h.tenant.GetActiveSigningKey(ctx)
	if err2 != nil {
		fmt.Printf("DEBUG: Hybrid tenant store also failed: %v\n", err2)
	} else {
		fmt.Printf("DEBUG: Hybrid tenant store success\n")
	}
	return res, err2
}

func (h *HybridSigningKeyStore) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	keys, err := h.global.ListPublicSigningKeys(ctx)
	if err == nil {
		return keys, nil
	}
	// Fallback to file store
	return h.tenant.ListPublicSigningKeys(ctx)
}

func (h *HybridSigningKeyStore) InsertSigningKey(ctx context.Context, k *core.SigningKey) error {
	// Try to write to global (DB)
	errGlobal := h.global.InsertSigningKey(ctx, k)

	// Always try to write to tenant (File) as backup/cache
	errTenant := h.tenant.InsertSigningKey(ctx, k)

	// If both fail, return error (prefer global error if set)
	if errGlobal != nil && errTenant != nil {
		return errGlobal
	}
	// If at least one succeeded, consider it success (resilience)
	return nil
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
