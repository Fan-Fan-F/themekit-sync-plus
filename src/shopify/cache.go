package shopify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// CacheFileName is the default filename used for caching theme asset metadata
const CacheFileName = ".themekit_cache.json"

// CacheEntry represents the cached metadata and checksum for an asset file
type CacheEntry struct {
	MTimeNano int64  `json:"mtime_nano"`
	Size      int64  `json:"size"`
	Checksum  string `json:"checksum"`
}

// ChecksumCache stores and retrieves asset checksums to avoid re-reading and re-hashing
// unchanged local theme files.
type ChecksumCache struct {
	mu       sync.RWMutex
	filePath string
	entries  map[string]CacheEntry
	dirty    bool
	disabled bool
}

// NewChecksumCache creates a new ChecksumCache for a project directory
func NewChecksumCache(directory string, disabled bool) *ChecksumCache {
	c := &ChecksumCache{
		filePath: filepath.Join(directory, CacheFileName),
		entries:  make(map[string]CacheEntry),
		disabled: disabled,
	}
	if !disabled {
		c.load()
	}
	return c
}

func (c *ChecksumCache) load() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return
	}

	var loaded map[string]CacheEntry
	if err := json.Unmarshal(data, &loaded); err == nil {
		c.entries = loaded
	}
}

// Save persists the cache to disk if it was modified
func (c *ChecksumCache) Save() error {
	if c == nil || c.disabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil
	}

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(c.filePath, data, 0644)
	if err == nil {
		c.dirty = false
	}
	return err
}

// Get attempts to find a matching checksum for key if mtime and size match
func (c *ChecksumCache) Get(key string, mtimeNano int64, size int64) (string, bool) {
	if c == nil || c.disabled {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}

	if entry.MTimeNano == mtimeNano && entry.Size == size && entry.Checksum != "" {
		return entry.Checksum, true
	}

	return "", false
}

// Set stores a checksum for key with its current mtime and size
func (c *ChecksumCache) Set(key string, mtimeNano int64, size int64, checksum string) {
	if c == nil || c.disabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if ok && entry.MTimeNano == mtimeNano && entry.Size == size && entry.Checksum == checksum {
		return
	}

	c.entries[key] = CacheEntry{
		MTimeNano: mtimeNano,
		Size:      size,
		Checksum:  checksum,
	}
	c.dirty = true
}

// Invalidate removes an entry from the cache
func (c *ChecksumCache) Invalidate(key string) {
	if c == nil || c.disabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[key]; ok {
		delete(c.entries, key)
		c.dirty = true
	}
}

// Count returns the number of entries in the cache
func (c *ChecksumCache) Count() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
