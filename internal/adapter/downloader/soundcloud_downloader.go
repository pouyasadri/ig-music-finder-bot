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

type SoundCloudDownloader struct{}

func NewSoundCloudDownloader() usecase.MusicDownloader {
	return &SoundCloudDownloader{}
}

func (d *SoundCloudDownloader) Download(ctx context.Context, targetDir, target string) (string, string, int, error) {
	outputTemplate := filepath.Join(targetDir, "track.%(ext)s")
	audioPath := filepath.Join(targetDir, "track.mp3")

	targetInput := target
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "scsearch") {
		targetInput = fmt.Sprintf("scsearch1:%s", target)
	}

	// SoundCloud downloads are much faster, 45s bounded timeout
	downloadCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(downloadCtx, "yt-dlp",
		"-f", "ba/b",
		"-N", "4",
		"--concurrent-fragments", "4",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "5",
		"--write-thumbnail",
		"--convert-thumbnails", "jpg",
		"--embed-metadata",
		"--embed-thumbnail",
		"--no-playlist",
		"--no-check-certificates",
		"--socket-timeout", "10",
		"-o", outputTemplate,
		targetInput,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", 0, fmt.Errorf("soundcloud download error: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return "", "", 0, fmt.Errorf("audio file not written: %w", err)
	}

	// Look for extracted thumbnail jpg
	thumbnailPath := filepath.Join(targetDir, "track.jpg")
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(targetDir, "track.png")); err == nil {
			thumbnailPath = filepath.Join(targetDir, "track.png")
		} else if _, err := os.Stat(filepath.Join(targetDir, "track.webp")); err == nil {
			thumbnailPath = filepath.Join(targetDir, "track.webp")
		} else {
			thumbnailPath = ""
		}
	}

	return audioPath, thumbnailPath, 0, nil
}
