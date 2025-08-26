package redis

import (
	"context"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

type Cache struct{ c *rdb.Client }

func New(addr string, db int) *Cache {
	return &Cache{c: rdb.NewClient(&rdb.Options{Addr: addr, DB: db})}
}

func (r *Cache) Get(k string) ([]byte, bool) {
	b, err := r.c.Get(context.Background(), k).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

func (r *Cache) Set(k string, v []byte, ttl time.Duration) {
	_ = r.c.Set(context.Background(), k, v, ttl).Err()
}

func (r *Cache) Delete(k string) { _ = r.c.Del(context.Background(), k).Err() }
