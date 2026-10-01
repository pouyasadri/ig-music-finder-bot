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
	cache         domain.MetadataCache
	cacheTTL      time.Duration
}

func NewCachedProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader, cache domain.MetadataCache, cacheTTL time.Duration) ReelAudioUseCase {
	uc := NewProcessReelUseCase(e, r, u, d).(*processReelUseCase)
	uc.cache = cache
	uc.cacheTTL = cacheTTL
	return uc
}

func NewProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader) ReelAudioUseCase {
	return &processReelUseCase{
		extractor:     e,
		recognizer:    r,
		urlRecognizer: u,
		downloader:    d,
	}
}

func (uc *processReelUseCase) Execute(ctx context.Context, targetDir, reelURL string, progressCb func(string)) (*domain.AudioPayload, error) {
	if progressCb != nil {
		progressCb("🔍 در حال استخراج ویدیو از اینستاگرام...")
	}
	t0 := time.Now()
	rawPath, snippetPath, err := uc.extractor.ExtractReel(ctx, targetDir, reelURL)
	extractMs := time.Since(t0).Milliseconds()
	if err != nil {
		slog.Error("media extraction failed", "extract_ms", extractMs, "err", err)
		return nil, fmt.Errorf("%w: %v", domain.ErrExtractionFailed, err)
	}

	if progressCb != nil {
		progressCb("🎵 در حال تشخیص آهنگ...")
	}
	t1 := time.Now()
	var meta *domain.TrackMetadata
	cacheKey := domain.NormalizeMediaURL(reelURL)
	if uc.cache != nil {
		if cached, cacheErr := uc.cache.Get(ctx, cacheKey); cacheErr == nil {
			var cachedMeta domain.TrackMetadata
			if jsonErr := json.Unmarshal(cached, &cachedMeta); jsonErr == nil && cachedMeta.IsMatched {
				meta = &cachedMeta
				slog.Info("recognition metadata cache hit", "key", cacheKey)
			}
		}
	}
	if meta == nil {
		meta, err = uc.recognizer.Identify(ctx, snippetPath)
	}
	recognizeMs := time.Since(t1).Milliseconds()

	// Tier 4 fallback if primary recognizers miss
	if (err != nil || meta == nil || !meta.IsMatched) && uc.urlRecognizer != nil {
		tUrl := time.Now()
		urlCtx, cancelUrl := context.WithTimeout(ctx, 3*time.Second)
		urlMeta, urlErr := uc.urlRecognizer.IdentifyByURL(urlCtx, reelURL)
		cancelUrl()

		if urlErr == nil && urlMeta != nil && urlMeta.IsMatched {
			meta = urlMeta
			err = nil
			slog.Info("url metadata scraping matched", "url_recognize_ms", time.Since(tUrl).Milliseconds())
		}
	}

	if err == nil && meta != nil && meta.IsMatched {
		if uc.cache != nil {
			if encoded, marshalErr := json.Marshal(meta); marshalErr == nil {
				if cacheErr := uc.cache.Set(ctx, cacheKey, encoded, time.Now().Add(uc.cacheTTL)); cacheErr != nil {
					slog.Warn("failed to cache recognition metadata", "err", cacheErr)
				}
			}
		}
		if progressCb != nil {
			progressCb(fmt.Sprintf("✅ آهنگ پیدا شد: %s\n⬇️ در حال دانلود کیفیت بالا...", meta.Title))
		}

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

			if progressCb != nil {
				progressCb("📤 در حال ارسال فایل صوتی...")
			}

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

	if progressCb != nil {
		progressCb("📤 آهنگ پیدا نشد. در حال ارسال صدای اصلی ویدیو...")
	}

	// Fallback response
	return &domain.AudioPayload{
		OriginalPath: rawPath,
		Title:        "Original Reel Audio",
		Performer:    "Instagram",
		FilePath:     rawPath,
		IsFullTrack:  false,
	}, nil
}
