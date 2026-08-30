package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"telegram-audio-bot/internal/usecase"
)

type YouTubeDownloader struct{}

func NewYouTubeDownloader() usecase.MusicDownloader {
	return &YouTubeDownloader{}
}

func (d *YouTubeDownloader) Download(ctx context.Context, targetDir, target string) (string, string, int, error) {
	// Clean output template for audio and thumbnail
	outputTemplate := filepath.Join(targetDir, "track.%(ext)s")
	audioPath := filepath.Join(targetDir, "track.mp3")

	targetInput := target
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "ytsearch") {
		targetInput = fmt.Sprintf("ytsearch1:%s", target)
	}

	// Bounded timeout context for YouTube search/download
	downloadCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(downloadCtx, "yt-dlp",
		"-f", "ba/b",
		"-N", "8",
		"--concurrent-fragments", "8",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "5",
		"--write-thumbnail",
		"--convert-thumbnails", "jpg",
		"--no-playlist",
		"--no-check-certificates",
		"--socket-timeout", "10",
		"--extractor-args", "youtube:player_client=android,web;player_skip=configs",
		"-o", outputTemplate,
		targetInput,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", 0, fmt.Errorf("yt-dlp download error: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return "", "", 0, fmt.Errorf("audio file not written: %w", err)
	}

	// Look for extracted thumbnail jpg
	thumbnailPath := filepath.Join(targetDir, "track.jpg")
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		// Fallback check for webp or png if conversion was skipped
		if _, err := os.Stat(filepath.Join(targetDir, "track.webp")); err == nil {
			thumbnailPath = filepath.Join(targetDir, "track.webp")
		} else {
			thumbnailPath = ""
		}
	}

	return audioPath, thumbnailPath, 0, nil
}
