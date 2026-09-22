package downloader

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

func (f *FallbackDownloader) Download(ctx context.Context, targetDir, target string, expectedDuration int) (string, string, int, error) {
	var lastErr error
	for _, d := range f.downloaders {
		if d.Downloader == nil {
			continue
		}

		attemptDir, err := os.MkdirTemp(targetDir, "download-attempt-*")
		if err != nil {
			lastErr = fmt.Errorf("create %s download directory: %w", d.Name, err)
			continue
		}
		audioPath, thumbPath, dur, err := d.Downloader.Download(ctx, attemptDir, target, expectedDuration)
		if err == nil && audioPath != "" {
			slog.Info("track downloaded successfully via engine", "engine", d.Name, "target", target)
			return audioPath, thumbPath, dur, nil
		}
		if err == nil {
			err = fmt.Errorf("engine returned an empty audio path")
		}

		lastErr = err
		slog.Warn("engine download failed, attempting next fallback", "engine", d.Name, "target", target, "err", err)
		if cleanupErr := os.RemoveAll(attemptDir); cleanupErr != nil {
			slog.Warn("failed to clean rejected download attempt", "engine", d.Name, "dir", filepath.Base(attemptDir), "err", cleanupErr)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no downloaders available")
	}
	return "", "", 0, fmt.Errorf("all downloaders failed: %w", lastErr)
}
