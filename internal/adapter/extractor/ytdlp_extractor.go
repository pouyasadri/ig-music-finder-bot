package extractor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"telegram-audio-bot/internal/usecase"
)

type YtDlpExtractor struct {
	cookiesPath string
}

func NewYtDlpExtractor(cookiesPath string) usecase.MediaExtractor {
	return &YtDlpExtractor{cookiesPath: cookiesPath}
}

func (e *YtDlpExtractor) ExtractReel(ctx context.Context, targetDir, url string) (string, string, error) {
	rawPath := filepath.Join(targetDir, "raw_reel.mp3")
	snippetPath := filepath.Join(targetDir, "snippet.mp3")

	// Set a bounded timeout context for reel extraction
	extractCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Download audio only (-f "ba/b") to save bandwidth and maximize speed
	args := []string{
		"-f", "ba/b",
		"-x",
		"--audio-format", "mp3",
		"--no-playlist",
		"-o", rawPath,
		url,
	}
	if _, err := os.Stat(e.cookiesPath); err == nil {
		args = append([]string{"--cookies", e.cookiesPath}, args...)
	}

	cmd := exec.CommandContext(extractCtx, "yt-dlp", args...)
	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	// Create 10s audio snippet using fast seek for ACRCloud fingerprinting
	ffCmd := exec.CommandContext(extractCtx, "ffmpeg", "-y", "-ss", "0", "-t", "10", "-i", rawPath, "-acodec", "copy", snippetPath)
	_ = ffCmd.Run()

	return rawPath, snippetPath, nil
}
