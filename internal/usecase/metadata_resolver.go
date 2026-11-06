package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

// TrackMetadataResolver resolves recognition metadata for a media URL + audio snippet.
type TrackMetadataResolver interface {
	Resolve(ctx context.Context, mediaURL, snippetPath string) (*domain.TrackMetadata, error)
}

type trackMetadataResolver struct {
	recognizer    domain.MusicRecognizer
	urlRecognizer domain.URLMusicRecognizer
	scrapeTimeout time.Duration
}

func NewTrackMetadataResolver(r domain.MusicRecognizer, u domain.URLMusicRecognizer, scrapeTimeout time.Duration) TrackMetadataResolver {
	if scrapeTimeout <= 0 {
		scrapeTimeout = 3 * time.Second
	}
	return &trackMetadataResolver{recognizer: r, urlRecognizer: u, scrapeTimeout: scrapeTimeout}
}

func (r *trackMetadataResolver) Resolve(ctx context.Context, mediaURL, snippetPath string) (*domain.TrackMetadata, error) {
	meta, err := r.recognizer.Identify(ctx, snippetPath)
	if err == nil && meta != nil && meta.IsMatched {
		return meta, nil
	}

	if r.urlRecognizer == nil {
		return meta, err
	}

	tURL := time.Now()
	urlCtx, cancel := context.WithTimeout(ctx, r.scrapeTimeout)
	urlMeta, urlErr := r.urlRecognizer.IdentifyByURL(urlCtx, mediaURL)
	cancel()

	if urlErr == nil && urlMeta != nil && urlMeta.IsMatched {
		slog.Info("url metadata scraping matched", "url_recognize_ms", time.Since(tURL).Milliseconds())
		return urlMeta, nil
	}
	return meta, err
}

// cachedTrackMetadataResolver decorates a resolver with URL-keyed metadata caching.
type cachedTrackMetadataResolver struct {
	inner    TrackMetadataResolver
	cache    domain.MetadataCache
	cacheTTL time.Duration
}

func NewCachedTrackMetadataResolver(inner TrackMetadataResolver, cache domain.MetadataCache, cacheTTL time.Duration) TrackMetadataResolver {
	if inner == nil {
		return nil
	}
	if cache == nil {
		return inner
	}
	return &cachedTrackMetadataResolver{inner: inner, cache: cache, cacheTTL: cacheTTL}
}

func (r *cachedTrackMetadataResolver) Resolve(ctx context.Context, mediaURL, snippetPath string) (*domain.TrackMetadata, error) {
	cacheKey := domain.NormalizeMediaURL(mediaURL)
	if cached, cacheErr := r.cache.Get(ctx, cacheKey); cacheErr == nil {
		var cachedMeta domain.TrackMetadata
		if jsonErr := json.Unmarshal(cached, &cachedMeta); jsonErr == nil && cachedMeta.IsMatched {
			slog.Info("recognition metadata cache hit", "key", cacheKey)
			return &cachedMeta, nil
		}
	}

	meta, err := r.inner.Resolve(ctx, mediaURL, snippetPath)
	if err != nil || meta == nil || !meta.IsMatched {
		return meta, err
	}

	if encoded, marshalErr := json.Marshal(meta); marshalErr == nil {
		if cacheErr := r.cache.Set(ctx, cacheKey, encoded, time.Now().Add(r.cacheTTL)); cacheErr != nil {
			slog.Warn("failed to cache recognition metadata", "err", cacheErr)
		}
	}
	return meta, nil
}
