package password

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Blacklist struct {
	mu   sync.RWMutex
	data map[string]struct{}
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
