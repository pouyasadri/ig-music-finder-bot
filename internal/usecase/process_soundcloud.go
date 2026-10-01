package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

type processSoundCloudUseCase struct {
	downloader domain.SoundCloudTrackDownloader
}

// NewProcessSoundCloudUseCase initializes a new SoundCloud audio use case
func NewProcessSoundCloudUseCase(d domain.SoundCloudTrackDownloader) SoundCloudAudioUseCase {
	return &processSoundCloudUseCase{
		downloader: d,
	}
}

func (uc *processSoundCloudUseCase) Execute(ctx context.Context, targetDir, soundCloudURL string, progress ProgressReporter) (*domain.AudioPayload, error) {
	reportProgress(progress, domain.ProgressDownloading, ProgressDetail{})

	t0 := time.Now()
	payload, err := uc.downloader.DownloadTrack(ctx, targetDir, soundCloudURL)
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
