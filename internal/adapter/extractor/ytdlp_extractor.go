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
	outputPattern := filepath.Join(targetDir, "raw_reel.%(ext)s")
	snippetPath := filepath.Join(targetDir, "snippet.mp3")

	// Set a bounded timeout context for reel extraction
	extractCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Download smallest media stream with 4 concurrent threads to maximize speed
	// Instagram lacks audio-only streams; using "ba/worst/b" downloads 240p/360p (~1-2MB) with identical 128k AAC audio instead of 1080p (~30-50MB)
	args := []string{
		"-f", "ba/worst/b",
		"-N", "4",
		"--concurrent-fragments", "4",
		"--no-playlist",
		"--no-check-certificates",
		"--socket-timeout", "10",
		"-o", outputPattern,
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

	// Locate the downloaded raw audio file
	matches, err := filepath.Glob(filepath.Join(targetDir, "raw_reel.*"))
	if err != nil || len(matches) == 0 {
		return "", "", fmt.Errorf("failed to locate extracted raw audio in %s", targetDir)
	}
	rawPath := matches[0]
	rawInfo, err := os.Stat(rawPath)
	if err != nil || rawInfo.IsDir() || rawInfo.Size() == 0 {
		return "", "", fmt.Errorf("extracted media is empty or unavailable: %s", rawPath)
	}

	// Create 10s audio snippet using fast seek for ACRCloud fingerprinting (only converts 10s)
	ffCmd := exec.CommandContext(extractCtx, "ffmpeg", "-y", "-ss", "0", "-t", "10", "-i", rawPath, "-vn", "-acodec", "libmp3lame", "-q:a", "4", snippetPath)
	if out, err := ffCmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("failed to create recognition snippet: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	snippetInfo, err := os.Stat(snippetPath)
	if err != nil || snippetInfo.IsDir() || snippetInfo.Size() == 0 {
		return "", "", fmt.Errorf("recognition snippet is empty or unavailable: %s", snippetPath)
	}

	return rawPath, snippetPath, nil
}
