package downloader

import (
	"context"
	"encoding/json"
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

type soundCloudInfoJSON struct {
	Title    string  `json:"title"`
	Uploader string  `json:"uploader"`
	Artist   string  `json:"artist"`
	Duration float64 `json:"duration"`
}

func (d *SoundCloudDownloader) Download(ctx context.Context, targetDir, target string) (string, string, int, error) {
	outputTemplate := filepath.Join(targetDir, "track.%(ext)s")
	audioPath := filepath.Join(targetDir, "track.mp3")

	targetInput := target
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "scsearch") {
		targetInput = fmt.Sprintf("scsearch3:%s", target)
	}

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
		"--write-info-json",
		"--no-playlist",
		"--no-check-certificates",
		"--socket-timeout", "10",
		"-o", outputTemplate,
		targetInput,
	)

	out, cmdErr := cmd.CombinedOutput()

	if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
		// Audio file downloaded successfully
	} else if cmdErr != nil {
		return "", "", 0, fmt.Errorf("soundcloud download error: %w (output: %s)", cmdErr, strings.TrimSpace(string(out)))
	} else {
		return "", "", 0, fmt.Errorf("audio file not written: %w", err)
	}

	thumbnailPath := findThumbnail(targetDir)

	duration := 0
	infoPath := filepath.Join(targetDir, "track.info.json")
	if infoBytes, err := os.ReadFile(infoPath); err == nil {
		var info soundCloudInfoJSON
		if err := json.Unmarshal(infoBytes, &info); err == nil {
			duration = int(info.Duration)
		}
	}

	return audioPath, thumbnailPath, duration, nil
}

func (d *SoundCloudDownloader) DownloadTrack(ctx context.Context, targetDir, url string) (*domain.AudioPayload, error) {
	audioPath, thumbnailPath, duration, err := d.Download(ctx, targetDir, url)
	if err != nil {
		return nil, err
	}

	var title, performer string
	infoPath := filepath.Join(targetDir, "track.info.json")
	if infoBytes, err := os.ReadFile(infoPath); err == nil {
		var info soundCloudInfoJSON
		if err := json.Unmarshal(infoBytes, &info); err == nil {
			title = strings.TrimSpace(info.Title)
			performer = strings.TrimSpace(info.Artist)
			if performer == "" {
				performer = strings.TrimSpace(info.Uploader)
			}
			if duration <= 0 && info.Duration > 0 {
				duration = int(info.Duration)
			}
		}
	}

	if title == "" {
		title = "SoundCloud Audio"
	}
	if performer == "" {
		performer = "SoundCloud"
	}

	return &domain.AudioPayload{
		Title:         title,
		Performer:     performer,
		FilePath:      audioPath,
		ThumbnailPath: thumbnailPath,
		Duration:      duration,
		IsFullTrack:   true,
		SoundCloudURL: url,
	}, nil
}

func findThumbnail(targetDir string) string {
	for _, ext := range []string{"jpg", "png", "webp"} {
		path := filepath.Join(targetDir, "track."+ext)
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			return path
		}
	}
	return ""
}
