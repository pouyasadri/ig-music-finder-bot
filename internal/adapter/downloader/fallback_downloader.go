package downloader

import (
	"context"
	"fmt"
	"log/slog"

	"telegram-audio-bot/internal/usecase"
)

type NamedDownloader struct {
	Name       string
	Downloader usecase.MusicDownloader
}

type FallbackDownloader struct {
	downloaders []NamedDownloader
}

func NewFallbackDownloader(downloaders ...NamedDownloader) usecase.MusicDownloader {
	return &FallbackDownloader{
		downloaders: downloaders,
	}
}

func (f *FallbackDownloader) Download(ctx context.Context, targetDir, target string) (string, string, int, error) {
	var lastErr error
	for _, d := range f.downloaders {
		if d.Downloader == nil {
			continue
		}

		audioPath, thumbPath, dur, err := d.Downloader.Download(ctx, targetDir, target)
		if err == nil && audioPath != "" {
			slog.Info("track downloaded successfully via engine", "engine", d.Name, "target", target)
			return audioPath, thumbPath, dur, nil
		}

		lastErr = err
		slog.Warn("engine download failed, attempting next fallback", "engine", d.Name, "target", target, "err", err)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no downloaders available")
	}
	return "", "", 0, fmt.Errorf("all downloaders failed: %w", lastErr)
}
