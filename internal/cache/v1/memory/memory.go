package memory

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type Cache struct{ c *gocache.Cache }

func New(defaultTTL time.Duration) *Cache {
	return &Cache{c: gocache.New(defaultTTL, time.Minute)}
}

func (m *Cache) Get(k string) ([]byte, bool) {
	v, ok := m.c.Get(k)
	if !ok {
		return nil, false
	}
	b, _ := v.([]byte)
	return b, true
}

func (m *Cache) Set(k string, v []byte, ttl time.Duration) { m.c.Set(k, v, ttl) }
func (m *Cache) Delete(k string)                           { m.c.Delete(k) }
