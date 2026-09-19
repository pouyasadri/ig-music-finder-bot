package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

type processReelUseCase struct {
	extractor     MediaExtractor
	recognizer    MusicRecognizer
	urlRecognizer URLMusicRecognizer
	downloader    MusicDownloader
}

func NewProcessReelUseCase(e MediaExtractor, r MusicRecognizer, u URLMusicRecognizer, d MusicDownloader) ReelAudioUseCase {
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
	rawPath, snippetPath, extractErr := uc.extractor.ExtractReel(ctx, targetDir, reelURL)
	extractMs := time.Since(t0).Milliseconds()

	var meta *domain.TrackMetadata
	var recognizeErr error
	var recognizeMs int64

	if extractErr != nil {
		slog.Error("media extraction failed", "extract_ms", extractMs, "err", extractErr)
		recognizeErr = extractErr // Treat it as if recognition failed to trigger the fallback
	} else {
		if progressCb != nil {
			progressCb("🎵 در حال تشخیص آهنگ...")
		}
		t1 := time.Now()
		meta, recognizeErr = uc.recognizer.Identify(ctx, snippetPath)
		recognizeMs = time.Since(t1).Milliseconds()
	}

	// Tier 4 fallback if primary recognizers miss (or if extraction failed)
	if (recognizeErr != nil || meta == nil || !meta.IsMatched) && uc.urlRecognizer != nil {
		tUrl := time.Now()
		urlCtx, cancelUrl := context.WithTimeout(ctx, 3*time.Second)
		urlMeta, urlErr := uc.urlRecognizer.IdentifyByURL(urlCtx, reelURL)
		cancelUrl()

		if urlErr == nil && urlMeta != nil && urlMeta.IsMatched {
			meta = urlMeta
			recognizeErr = nil
			slog.Info("url metadata scraping matched", "url_recognize_ms", time.Since(tUrl).Milliseconds())
		}
	}

	if recognizeErr == nil && meta != nil && meta.IsMatched {
		if progressCb != nil {
			progressCb(fmt.Sprintf("✅ آهنگ پیدا شد: %s\n⬇️ در حال دانلود کیفیت بالا...", meta.Title))
		}
		slog.Info("track recognized", "title", meta.Title, "artist", meta.Artist, "extract_ms", extractMs, "recognize_ms", recognizeMs)

		downloadTarget := fmt.Sprintf("%s %s", meta.Title, meta.Artist)
		if meta.Artist == "" {
			downloadTarget = meta.Title
		}

		t2 := time.Now()
		fullTrackPath, thumbnailPath, duration, downloadErr := uc.downloader.Download(ctx, targetDir, downloadTarget)
		downloadMs := time.Since(t2).Milliseconds()

		if downloadErr == nil && fullTrackPath != "" {
			slog.Info("download completed", "download_ms", downloadMs, "target", downloadTarget)

			finalDuration := duration
			if finalDuration <= 0 {
				finalDuration = meta.Duration
			}

			if progressCb != nil {
				progressCb("📤 در حال ارسال فایل صوتی...")
			}

			return &domain.AudioPayload{
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
		slog.Warn("download failed, falling back to raw reel audio", "download_ms", downloadMs, "err", downloadErr)
	} else {
		slog.Info("unrecognized sound, using raw reel audio", "extract_ms", extractMs, "recognize_ms", recognizeMs)
	}

	if extractErr != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrExtractionFailed, extractErr)
	}

	if progressCb != nil {
		progressCb("📤 آهنگ پیدا نشد. در حال ارسال صدای اصلی ویدیو...")
	}

	// Fallback response
	return &domain.AudioPayload{
		Title:       "Original Reel Audio",
		Performer:   "Instagram",
		FilePath:    rawPath,
		IsFullTrack: false,
	}, nil
}
