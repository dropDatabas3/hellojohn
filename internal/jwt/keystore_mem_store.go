package jwt

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type MemorySigningKeyStore struct {
	mu   sync.RWMutex
	list []core.SigningKey
}

func NewMemorySigningKeyStore() *MemorySigningKeyStore { return &MemorySigningKeyStore{} }

func (m *MemorySigningKeyStore) GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now().UTC()
	var act *core.SigningKey
	for i := range m.list {
		k := &m.list[i]
		if k.Status == core.KeyActive && !k.NotBefore.After(now) {
			if act == nil || k.NotBefore.After(act.NotBefore) {
				act = k
			}
		}
	}
	if act == nil {
		return nil, core.ErrNotFound
	}
	cp := *act
	return &cp, nil
}

func (m *MemorySigningKeyStore) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]core.SigningKey, 0, len(m.list))
	for _, k := range m.list {
		if k.Status == core.KeyActive || k.Status == core.KeyRetiring {
			cp := k
			// no exponemos private_key
			cp.PrivateKey = nil
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == core.KeyActive
		}
		return out[i].NotBefore.After(out[j].NotBefore)
	})
	return out, nil
}

func (m *MemorySigningKeyStore) InsertSigningKey(ctx context.Context, k *core.SigningKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.list = append(m.list, *k)
	return nil
}
