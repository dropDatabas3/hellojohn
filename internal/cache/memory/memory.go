package memory

import (
	"time"

	"github.com/dropDatabas3/hellojohn/internal/cache"
	gocache "github.com/patrickmn/go-cache"
)

type Mem struct{ c *gocache.Cache }

func New(defaultTTL time.Duration) cache.Cache {
	return &Mem{c: gocache.New(defaultTTL, time.Minute)}
}

func (m *Mem) Get(k string) ([]byte, bool) {
	v, ok := m.c.Get(k)
	if !ok {
		return nil, false
	}
	b, _ := v.([]byte)
	return b, true
}

func (m *Mem) Set(k string, v []byte, ttl time.Duration) { m.c.Set(k, v, ttl) }
func (m *Mem) Delete(k string)                           { m.c.Delete(k) }
