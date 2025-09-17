package password

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

type Blacklist struct {
	mu   sync.RWMutex
	data map[string]struct{}
}

// global cached instance (lazy) to avoid re-reading the file on every request
var cached atomic.Pointer[Blacklist]
var loadOnce sync.Once
var cachedPath atomic.Pointer[string]

// GetCachedBlacklist returns a singleton blacklist for the provided path.
// If path changes between calls, it reloads once for the new path.
func GetCachedBlacklist(path string) (*Blacklist, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		// empty path => always return empty blacklist (no caching required)
		bl := cached.Load()
		if bl != nil {
			return bl, nil
		}
		empty := &Blacklist{data: map[string]struct{}{}}
		if cached.CompareAndSwap(nil, empty) {
			return empty, nil
		}
		return cached.Load(), nil
	}

	// If path matches existing cached path, just return.
	cp := cachedPath.Load()
	if cp != nil && *cp == p {
		if cur := cached.Load(); cur != nil {
			return cur, nil
		}
	}

	// Reload (not strictly single-flight; acceptable for startup race)
	bl, err := LoadBlacklist(p)
	if err != nil {
		return nil, err
	}
	cached.Store(bl)
	cachedPath.Store(&p)
	return bl, nil
}

func LoadBlacklist(path string) (*Blacklist, error) {
	bl := &Blacklist{data: map[string]struct{}{}}
	if strings.TrimSpace(path) == "" {
		return bl, nil
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.TrimSpace(strings.ToLower(sc.Text()))
		if s != "" && !strings.HasPrefix(s, "#") {
			bl.data[s] = struct{}{}
		}
	}
	return bl, sc.Err()
}

func (b *Blacklist) Contains(pwd string) bool {
	if b == nil {
		return false
	}
	p := strings.ToLower(strings.TrimSpace(pwd))
	b.mu.RLock()
	_, ok := b.data[p]
	b.mu.RUnlock()
	return ok
}
