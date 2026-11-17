package shopify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecksumCache_Basic(t *testing.T) {
	tempDir := t.TempDir()

	cache := NewChecksumCache(tempDir, false)
	assert.Equal(t, 0, cache.Count())

	// Setting an entry
	cache.Set("assets/test.js", 123456789, 512, "abc123md5")
	assert.Equal(t, 1, cache.Count())

	// Hit with same mtime and size
	cs, ok := cache.Get("assets/test.js", 123456789, 512)
	assert.True(t, ok)
	assert.Equal(t, "abc123md5", cs)

	// Miss with different mtime
	_, ok = cache.Get("assets/test.js", 999999999, 512)
	assert.False(t, ok)

	// Miss with different size
	_, ok = cache.Get("assets/test.js", 123456789, 1024)
	assert.False(t, ok)

	// Miss on non-existent key
	_, ok = cache.Get("assets/missing.js", 123456789, 512)
	assert.False(t, ok)

	// Persist and reload
	err := cache.Save()
	assert.NoError(t, err)

	cacheFile := filepath.Join(tempDir, CacheFileName)
	assert.FileExists(t, cacheFile)

	loadedCache := NewChecksumCache(tempDir, false)
	assert.Equal(t, 1, loadedCache.Count())
	cs2, ok2 := loadedCache.Get("assets/test.js", 123456789, 512)
	assert.True(t, ok2)
	assert.Equal(t, "abc123md5", cs2)

	// Invalidate
	loadedCache.Invalidate("assets/test.js")
	assert.Equal(t, 0, loadedCache.Count())
	_ = loadedCache.Save()
}

func TestChecksumCache_Disabled(t *testing.T) {
	tempDir := t.TempDir()

	cache := NewChecksumCache(tempDir, true)
	cache.Set("assets/test.js", 123456789, 512, "abc123md5")
	assert.Equal(t, 0, cache.Count())

	_, ok := cache.Get("assets/test.js", 123456789, 512)
	assert.False(t, ok)

	err := cache.Save()
	assert.NoError(t, err)

	cacheFile := filepath.Join(tempDir, CacheFileName)
	_, statErr := os.Stat(cacheFile)
	assert.True(t, os.IsNotExist(statErr))
}
