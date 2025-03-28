package docusign

import (
	"sync"
	"time"
)

type TokenCache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V)
	Start()
	Stop()
}

type KeysCache struct {
	entries  map[DocusignUser]DocusignUserCacheEntry
	interval time.Duration
	mu       sync.RWMutex
	done     chan struct{}
}

func NewKeysCache(interval time.Duration) *KeysCache {
	cache := &KeysCache{
		entries:  make(map[DocusignUser]DocusignUserCacheEntry),
		interval: interval,
		mu:       sync.RWMutex{},
		done:     make(chan struct{}),
	}

	cache.Start()

	return cache
}

func (c *KeysCache) Get(user DocusignUser) (DocusignUserCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[user]
	return entry, ok
}

func (c *KeysCache) Set(user DocusignUser, entry DocusignUserCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[user] = entry
}

func (c *KeysCache) Start() {
	ticker := time.NewTicker(c.interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.mu.Lock()
				for k, v := range c.entries {
					if time.Since(v.CreatedAt) > v.TTL {
						delete(c.entries, k)
					}
				}
				c.mu.Unlock()
			case <-c.done:
				return
			}
		}
	}()
}

func (c *KeysCache) Stop() {
	close(c.done)
}
