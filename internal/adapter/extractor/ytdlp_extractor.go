package extractor

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
	}

	// Only attach cookies if the file exists and is not empty
	if fi, err := os.Stat(e.cookiesPath); err == nil && fi.Size() > 0 {
		args = append(args, "--cookies", e.cookiesPath)
	}

	args = append(args, url)

	cmd := exec.CommandContext(extractCtx, "yt-dlp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("yt-dlp extract error: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	// Create 10s audio snippet using fast seek for ACRCloud fingerprinting
	ffCmd := exec.CommandContext(extractCtx, "ffmpeg", "-y", "-ss", "0", "-t", "10", "-i", rawPath, "-acodec", "copy", snippetPath)
	_ = ffCmd.Run()

	return rawPath, snippetPath, nil
}
