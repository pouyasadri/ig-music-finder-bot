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
	out, probeErr := cmd.Output()
	if probeErr != nil {
		return 0, nil
	}
	seconds, parseErr := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if parseErr != nil || seconds <= 0 {
		return 0, nil
	}
	return int(seconds + 0.5), nil
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
