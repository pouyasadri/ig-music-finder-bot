package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

type processReelUseCase struct {
	extractor     domain.MediaExtractor
	recognizer    domain.MusicRecognizer
	urlRecognizer domain.URLMusicRecognizer
	downloader    domain.MusicDownloader
}

// cachedProcessReelUseCase decorates reel processing with recognition metadata caching.
// File paths are request-scoped, so only TrackMetadata is cached — never AudioPayload.
type cachedProcessReelUseCase struct {
	inner    *processReelUseCase
	cache    domain.MetadataCache
	cacheTTL time.Duration
}

func NewProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader) ReelAudioUseCase {
	return newProcessReelUseCase(e, r, u, d)
}

func NewCachedProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader, cache domain.MetadataCache, cacheTTL time.Duration) ReelAudioUseCase {
	inner := newProcessReelUseCase(e, r, u, d)
	if cache == nil {
		return inner
	}
	return &cachedProcessReelUseCase{
		inner:    inner,
		cache:    cache,
		cacheTTL: cacheTTL,
	}
}

func newProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader) *processReelUseCase {
	return &processReelUseCase{
		extractor:     e,
		recognizer:    r,
		urlRecognizer: u,
		downloader:    d,
	}
}

func (uc *cachedProcessReelUseCase) Execute(ctx context.Context, targetDir, reelURL string, progress ProgressReporter) (*domain.AudioPayload, error) {
	return uc.inner.execute(ctx, targetDir, reelURL, progress, uc.cache, uc.cacheTTL)
}

func (uc *processReelUseCase) Execute(ctx context.Context, targetDir, reelURL string, progress ProgressReporter) (*domain.AudioPayload, error) {
	return uc.execute(ctx, targetDir, reelURL, progress, nil, 0)
}

func (uc *processReelUseCase) execute(ctx context.Context, targetDir, reelURL string, progress ProgressReporter, cache domain.MetadataCache, cacheTTL time.Duration) (*domain.AudioPayload, error) {
	reportProgress(progress, domain.ProgressExtracting, ProgressDetail{})

	t0 := time.Now()
	rawPath, snippetPath, err := uc.extractor.ExtractReel(ctx, targetDir, reelURL)
	extractMs := time.Since(t0).Milliseconds()
	if err != nil {
		slog.Error("media extraction failed", "extract_ms", extractMs, "err", err)
		return nil, fmt.Errorf("%w: %v", domain.ErrExtractionFailed, err)
	}

	reportProgress(progress, domain.ProgressRecognizing, ProgressDetail{})

	t1 := time.Now()
	meta, err := uc.resolveMetadata(ctx, reelURL, snippetPath, cache)
	recognizeMs := time.Since(t1).Milliseconds()

	if err == nil && meta != nil && meta.IsMatched {
		if cache != nil {
			if encoded, marshalErr := json.Marshal(meta); marshalErr == nil {
				if cacheErr := cache.Set(ctx, domain.NormalizeMediaURL(reelURL), encoded, time.Now().Add(cacheTTL)); cacheErr != nil {
					slog.Warn("failed to cache recognition metadata", "err", cacheErr)
				}
			}
		}

		reportProgress(progress, domain.ProgressDownloading, ProgressDetail{
			Title:   meta.Title,
			Artist:  meta.Artist,
			Matched: true,
		})

		slog.Info("track recognized", "title", meta.Title, "artist", meta.Artist, "extract_ms", extractMs, "recognize_ms", recognizeMs)

		downloadTarget := fmt.Sprintf("%s %s", meta.Title, meta.Artist)
		if meta.Artist == "" {
			downloadTarget = meta.Title
		}

		t2 := time.Now()
		fullTrackPath, thumbnailPath, duration, err := uc.downloader.Download(ctx, targetDir, downloadTarget, meta.Duration)
		downloadMs := time.Since(t2).Milliseconds()

		if err == nil && fullTrackPath != "" && domain.DefaultDurationPolicy().Matches(duration, meta.Duration) {
			slog.Info("download completed", "download_ms", downloadMs, "target", downloadTarget)

			finalDuration := duration
			if finalDuration <= 0 {
				finalDuration = meta.Duration
			}

			reportProgress(progress, domain.ProgressSending, ProgressDetail{
				Title:   meta.Title,
				Artist:  meta.Artist,
				Matched: true,
			})

			return &domain.AudioPayload{
				OriginalPath:  rawPath,
				Title:         meta.Title,
				Performer:     meta.Artist,
				FilePath:      fullTrackPath,
				ThumbnailPath: thumbnailPath,
				Duration:      finalDuration,
				IsFullTrack:   true,
				SpotifyURL:    meta.SpotifyURL,
				YouTubeURL:    meta.YouTubeURL,
			}, nil
		}

		if err == nil {
			err = fmt.Errorf("%w: actual=%ds expected=%ds", domain.ErrDurationMismatch, duration, meta.Duration)
		}
		if err != nil && !errors.Is(err, domain.ErrDurationMismatch) {
			err = fmt.Errorf("%w: %v", domain.ErrDownloadFailed, err)
		}
		slog.Warn("download failed, falling back to raw reel audio", "download_ms", downloadMs, "err", err)
	} else {
		slog.Info("unrecognized sound, using raw reel audio", "extract_ms", extractMs, "recognize_ms", recognizeMs)
	}

	reportProgress(progress, domain.ProgressSending, ProgressDetail{Fallback: true})

	return &domain.AudioPayload{
		OriginalPath: rawPath,
		Title:        "Original Reel Audio",
		Performer:    "Instagram",
		FilePath:     rawPath,
		IsFullTrack:  false,
	}, nil
}

func (uc *processReelUseCase) resolveMetadata(ctx context.Context, reelURL, snippetPath string, cache domain.MetadataCache) (*domain.TrackMetadata, error) {
	cacheKey := domain.NormalizeMediaURL(reelURL)
	if cache != nil {
		if cached, cacheErr := cache.Get(ctx, cacheKey); cacheErr == nil {
			var cachedMeta domain.TrackMetadata
			if jsonErr := json.Unmarshal(cached, &cachedMeta); jsonErr == nil && cachedMeta.IsMatched {
				slog.Info("recognition metadata cache hit", "key", cacheKey)
				return &cachedMeta, nil
			}
		}
	}

	meta, err := uc.recognizer.Identify(ctx, snippetPath)
	if err == nil && meta != nil && meta.IsMatched {
		return meta, nil
	}

	if uc.urlRecognizer == nil {
		return meta, err
	}

	tUrl := time.Now()
	urlCtx, cancelUrl := context.WithTimeout(ctx, 3*time.Second)
	urlMeta, urlErr := uc.urlRecognizer.IdentifyByURL(urlCtx, reelURL)
	cancelUrl()

	if urlErr == nil && urlMeta != nil && urlMeta.IsMatched {
		slog.Info("url metadata scraping matched", "url_recognize_ms", time.Since(tUrl).Milliseconds())
		return urlMeta, nil
	}
	return meta, err
}
