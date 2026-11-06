package sqlite

import (
	"context"
	"time"

	"telegram-audio-bot/internal/domain"
)

type MetadataCacheAdapter struct {
	cache *SQLiteCacheRepository
}

func NewMetadataCacheAdapter(cache *SQLiteCacheRepository) *MetadataCacheAdapter {
	return &MetadataCacheAdapter{cache: cache}
}

var _ domain.MetadataCache = (*MetadataCacheAdapter)(nil)

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
