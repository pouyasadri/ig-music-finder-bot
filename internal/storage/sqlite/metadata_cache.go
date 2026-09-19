package sqlite

import (
	"context"
	"time"
)

type MetadataCacheAdapter struct {
	cache Cache
}

func NewMetadataCacheAdapter(cache Cache) *MetadataCacheAdapter {
	return &MetadataCacheAdapter{cache: cache}
}

func (c *MetadataCacheAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	entry, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return entry.Value, nil
}

func (c *MetadataCacheAdapter) Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error {
	return c.cache.Set(ctx, CacheEntry{Key: key, Value: value, ExpiresAt: expiresAt})
}
