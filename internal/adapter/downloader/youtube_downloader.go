package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"telegram-audio-bot/internal/usecase"
)

type YouTubeDownloader struct{}

func NewYouTubeDownloader() usecase.MusicDownloader {
	return &YouTubeDownloader{}
}

func (d *YouTubeDownloader) Download(ctx context.Context, targetDir, query string) (string, error) {
	outputPath := filepath.Join(targetDir, "full_track.mp3")
	searchQuery := fmt.Sprintf("ytsearch1:%s", query)

	// Bounded timeout context for YouTube search and download
	downloadCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(downloadCtx, "yt-dlp",
		"-f", "ba/b",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--embed-metadata",
		"--embed-thumbnail",
		"--no-playlist",
		"--extractor-args", "youtube:player_client=android,web;player_skip=configs",
		"-o", outputPath,
		searchQuery,
	)

	if err := cmd.Run(); err != nil {
		return "", err
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not written: %w", err)
	}

	return outputPath, nil
}
