package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SoundCloudDownloader struct{}

func NewSoundCloudDownloader() *SoundCloudDownloader {
	return &SoundCloudDownloader{}
}

func (d *SoundCloudDownloader) DownloadTrack(ctx context.Context, targetDir, url string) (*domain.AudioPayload, error) {
	filePath, thumbnailPath, duration, err := d.Download(ctx, targetDir, url, 0)
	if err != nil {
		return nil, err
	}
	return &domain.AudioPayload{
		FilePath:      filePath,
		ThumbnailPath: thumbnailPath,
		Duration:      duration,
		IsFullTrack:   true,
		SoundCloudURL: url,
	}, nil
}

func (d *SoundCloudDownloader) Download(ctx context.Context, targetDir, target string, expectedDuration int) (string, string, int, error) {
	outputTemplate := filepath.Join(targetDir, "track.%(ext)s")
	audioPath := filepath.Join(targetDir, "track.mp3")

	targetInput := target
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "scsearch") {
		targetInput = fmt.Sprintf("scsearch3:%s", target)
	}

	// SoundCloud downloads are much faster, 45s bounded timeout
	downloadCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(downloadCtx, "yt-dlp",
		"-f", "ba/b",
		"-N", "4",
		"--concurrent-fragments", "4",
		"--max-downloads", "1",
		"--no-abort-on-error",
		"--match-filter", "!drm",
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

	out, cmdErr := cmd.CombinedOutput()

	// Verify if audio file was written successfully (even if yt-dlp exited with status 101 for --max-downloads)
	if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
		// Success!
	} else if cmdErr != nil {
		return "", "", 0, fmt.Errorf("soundcloud download error: %w (output: %s)", cmdErr, strings.TrimSpace(string(out)))
	} else {
		return "", "", 0, fmt.Errorf("audio file not written: %w", err)
	}
	duration, err := validateAudioFile(audioPath, 49*1024*1024)
	if err != nil {
		return "", "", 0, err
	}
	if err := validateTrackDuration(duration, expectedDuration); err != nil {
		return "", "", 0, fmt.Errorf("soundcloud result rejected: %w", err)
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
	thumbnailPath = validateThumbnail(thumbnailPath)

	return audioPath, thumbnailPath, duration, nil
}
