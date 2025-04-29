package teams

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

type DiskCache struct {
	path string
	mu   sync.RWMutex
}

func NewDiskCache(filePath string) *DiskCache {
	return &DiskCache{path: filePath}
}

func (d *DiskCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, err := os.Stat(d.path); os.IsNotExist(err) {
		return nil // no cache yet
	}

	data, err := os.ReadFile(d.path)
	if err != nil {
		return err
	}

	return cache.Unmarshal(data)
}

func (d *DiskCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dir := filepath.Dir(d.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := cache.Marshal()
	if err != nil {
		return err
	}

	return os.WriteFile(d.path, data, 0600)
}
