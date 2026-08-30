package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"telegram-audio-bot/internal/usecase"
)

type YouTubeDownloader struct{}

func NewYouTubeDownloader() usecase.MusicDownloader {
	return &YouTubeDownloader{}
}

func (d *YouTubeDownloader) Download(ctx context.Context, targetDir, query string) (string, string, int, error) {
	// Clean output template for audio and thumbnail
	outputTemplate := filepath.Join(targetDir, "track.%(ext)s")
	audioPath := filepath.Join(targetDir, "track.mp3")
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
		"--write-thumbnail",
		"--convert-thumbnails", "jpg",
		"--no-playlist",
		"--extractor-args", "youtube:player_client=android,web;player_skip=configs",
		"-o", outputTemplate,
		searchQuery,
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

	// Inspect duration via ffprobe
	duration := getAudioDuration(downloadCtx, audioPath)

	return audioPath, thumbnailPath, duration, nil
}

func getAudioDuration(ctx context.Context, filePath string) int {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	floatStr := strings.TrimSpace(string(out))
	val, err := strconv.ParseFloat(floatStr, 64)
	if err != nil {
		return 0
	}
	return int(val)
}
