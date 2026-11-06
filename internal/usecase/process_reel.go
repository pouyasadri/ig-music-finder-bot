package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

type processReelUseCase struct {
	extractor  domain.MediaExtractor
	resolver   TrackMetadataResolver
	downloader domain.MusicDownloader
}

func NewProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader) ReelAudioUseCase {
	return NewProcessReelUseCaseWithResolver(e, NewTrackMetadataResolver(r, u), d)
}

func NewProcessReelUseCaseWithResolver(e domain.MediaExtractor, resolver TrackMetadataResolver, d domain.MusicDownloader) ReelAudioUseCase {
	return &processReelUseCase{
		extractor:  e,
		resolver:   resolver,
		downloader: d,
	}
}

// NewCachedProcessReelUseCase decorates recognition with metadata caching via
// NewCachedTrackMetadataResolver — an explicit decorator with no type assertions.
func NewCachedProcessReelUseCase(e domain.MediaExtractor, r domain.MusicRecognizer, u domain.URLMusicRecognizer, d domain.MusicDownloader, cache domain.MetadataCache, cacheTTL time.Duration) ReelAudioUseCase {
	resolver := NewCachedTrackMetadataResolver(NewTrackMetadataResolver(r, u), cache, cacheTTL)
	return NewProcessReelUseCaseWithResolver(e, resolver, d)
}

func (uc *processReelUseCase) Execute(ctx context.Context, targetDir, reelURL string, progress ProgressReporter) (*domain.AudioPayload, error) {
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
	var meta *domain.TrackMetadata
	if uc.resolver != nil {
		meta, err = uc.resolver.Resolve(ctx, reelURL, snippetPath)
	}
	recognizeMs := time.Since(t1).Milliseconds()

	if err == nil && meta != nil && meta.IsMatched {
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
