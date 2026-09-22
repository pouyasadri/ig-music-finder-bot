package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
		return 0, fmt.Errorf("audio file exceeds limit: %d bytes", info.Size())
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
	if actual <= 0 {
		return fmt.Errorf("audio duration is unavailable")
	}
	if expected <= 0 {
		if actual < 60 {
			return fmt.Errorf("audio appears to be a preview: %d seconds", actual)
		}
		return nil
	}

	// Providers may add intros/outros, but a result substantially shorter than
	// the recognized recording is almost always a preview or an incomplete file.
	tolerance := expected / 10
	if tolerance < 10 {
		tolerance = 10
	}
	if actual+tolerance < expected {
		return fmt.Errorf("audio is shorter than expected: actual=%ds expected=%ds", actual, expected)
	}
	if actual > expected+tolerance*3 {
		return fmt.Errorf("audio is longer than expected: actual=%ds expected=%ds", actual, expected)
	}
	return nil
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
