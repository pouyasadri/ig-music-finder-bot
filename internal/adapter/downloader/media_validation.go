package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"telegram-audio-bot/internal/domain"
)

func validateAudioFile(path string, maxBytes int64) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("audio file unavailable: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return 0, fmt.Errorf("audio file is empty: %s", path)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return 0, fmt.Errorf("%w: %d bytes", domain.ErrFileTooLarge, info.Size())
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, probeErr := cmd.CombinedOutput()
	if probeErr != nil {
		return 0, fmt.Errorf("failed to probe audio duration for %s: %w (output: %s)", path, probeErr, strings.TrimSpace(string(out)))
	}
	seconds, parseErr := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if parseErr != nil || seconds <= 0 {
		return 0, fmt.Errorf("invalid audio duration for %s: %q", path, strings.TrimSpace(string(out)))
	}
	return int(seconds + 0.5), nil
}

func validateTrackDuration(actual, expected int) error {
	return domain.DefaultDurationPolicy().Validate(actual, expected)
}

func validateThumbnail(path string) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return ""
	}
	return path
}
