package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

type processMediaUseCase struct {
	extractor            domain.MediaExtractor
	resolver             TrackMetadataResolver
	downloader           domain.MusicDownloader
	soundCloudDownloader domain.SoundCloudTrackDownloader
}

func NewProcessMediaUseCase(
	extractor domain.MediaExtractor,
	resolver TrackMetadataResolver,
	downloader domain.MusicDownloader,
	soundCloudDownloader domain.SoundCloudTrackDownloader,
) ProcessMediaUseCase {
	return &processMediaUseCase{
		extractor:            extractor,
		resolver:             resolver,
		downloader:           downloader,
		soundCloudDownloader: soundCloudDownloader,
	}
}

// NewProcessMediaUseCaseFromRecognizers is a convenience constructor used by tests and simpler wiring.
func NewProcessMediaUseCaseFromRecognizers(
	extractor domain.MediaExtractor,
	recognizer domain.MusicRecognizer,
	urlRecognizer domain.URLMusicRecognizer,
	downloader domain.MusicDownloader,
	soundCloudDownloader domain.SoundCloudTrackDownloader,
) ProcessMediaUseCase {
	return NewProcessMediaUseCase(extractor, NewTrackMetadataResolver(recognizer, urlRecognizer, 0), downloader, soundCloudDownloader)
}

// NewCachedProcessMediaUseCase wires a cached metadata resolver into ProcessMedia.
func NewCachedProcessMediaUseCase(
	extractor domain.MediaExtractor,
	recognizer domain.MusicRecognizer,
	urlRecognizer domain.URLMusicRecognizer,
	downloader domain.MusicDownloader,
	soundCloudDownloader domain.SoundCloudTrackDownloader,
	cache domain.MetadataCache,
	cacheTTL time.Duration,
) ProcessMediaUseCase {
	resolver := NewCachedTrackMetadataResolver(NewTrackMetadataResolver(recognizer, urlRecognizer, 0), cache, cacheTTL)
	return NewProcessMediaUseCase(extractor, resolver, downloader, soundCloudDownloader)
}

func (uc *processMediaUseCase) Execute(ctx context.Context, in ProcessMediaInput, progress ProgressReporter) (*domain.AudioPayload, error) {
	kind := in.Kind
	if kind == "" {
		kind = InferSourceKind(in.SourceURL)
	}
	switch kind {
	case domain.SourceKindSoundCloud:
		return uc.executeSoundCloud(ctx, in.WorkDir, in.SourceURL, progress)
	default:
		return uc.executeRecognizedMedia(ctx, in.WorkDir, in.SourceURL, kind, progress)
	}
}

func (uc *processMediaUseCase) executeSoundCloud(ctx context.Context, targetDir, soundCloudURL string, progress ProgressReporter) (*domain.AudioPayload, error) {
	if uc.soundCloudDownloader == nil {
		return nil, fmt.Errorf("%w: soundcloud processing is not configured", domain.ErrInvalidInput)
	}

	reportProgress(progress, domain.ProgressDownloading, ProgressDetail{})

	t0 := time.Now()
	payload, err := uc.soundCloudDownloader.DownloadTrack(ctx, targetDir, soundCloudURL)
	downloadMs := time.Since(t0).Milliseconds()

	if err != nil {
		slog.Error("soundcloud track download failed", "download_ms", downloadMs, "err", err, "url", soundCloudURL)
		return nil, fmt.Errorf("%w: %v", domain.ErrSoundCloudDownloadFailed, err)
	}
	if payload == nil || payload.FilePath == "" {
		slog.Error("soundcloud track payload empty", "download_ms", downloadMs, "url", soundCloudURL)
		return nil, fmt.Errorf("%w: empty audio payload", domain.ErrSoundCloudDownloadFailed)
	}

	payload.IsFullTrack = true
	if payload.SoundCloudURL == "" {
		payload.SoundCloudURL = soundCloudURL
	}
	if payload.Title == "" {
		payload.Title = "SoundCloud Audio"
	}
	if payload.Performer == "" {
		payload.Performer = "SoundCloud"
	}

	slog.Info("soundcloud track processed successfully",
		"title", payload.Title,
		"artist", payload.Performer,
		"duration", payload.Duration,
		"download_ms", downloadMs,
	)

	reportProgress(progress, domain.ProgressSending, ProgressDetail{
		Title:   payload.Title,
		Artist:  payload.Performer,
		Matched: true,
	})
	return payload, nil
}

func fallbackSourceLabels(kind domain.SourceKind) (title, performer string) {
	switch kind {
	case domain.SourceKindUpload:
		return "Original Audio", "Upload"
	case domain.SourceKindInstagram:
		return "Original Audio", "Instagram"
	default:
		return "Original Audio", "Media"
	}
}

func (uc *processMediaUseCase) executeRecognizedMedia(ctx context.Context, targetDir, mediaURL string, kind domain.SourceKind, progress ProgressReporter) (*domain.AudioPayload, error) {
	reportProgress(progress, domain.ProgressExtracting, ProgressDetail{})

	t0 := time.Now()
	rawPath, snippetPath, err := uc.extractor.ExtractReel(ctx, targetDir, mediaURL)
	extractMs := time.Since(t0).Milliseconds()
	if err != nil {
		slog.Error("media extraction failed", "extract_ms", extractMs, "err", err)
		return nil, fmt.Errorf("%w: %v", domain.ErrExtractionFailed, err)
	}

	reportProgress(progress, domain.ProgressRecognizing, ProgressDetail{})

	t1 := time.Now()
	var meta *domain.TrackMetadata
	if uc.resolver != nil {
		meta, err = uc.resolver.Resolve(ctx, mediaURL, snippetPath)
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
		slog.Warn("download failed, falling back to original audio", "download_ms", downloadMs, "err", err)
	} else {
		slog.Info("unrecognized sound, using original audio", "extract_ms", extractMs, "recognize_ms", recognizeMs)
	}

	reportProgress(progress, domain.ProgressSending, ProgressDetail{Fallback: true})
	title, performer := fallbackSourceLabels(kind)
	return &domain.AudioPayload{
		OriginalPath: rawPath,
		Title:        title,
		Performer:    performer,
		FilePath:     rawPath,
		IsFullTrack:  false,
	}, nil
}
