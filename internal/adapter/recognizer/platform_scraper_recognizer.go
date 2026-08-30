package recognizer

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

type PlatformScraperRecognizer struct {
	cookiesPath string
}

func NewPlatformScraperRecognizer(cookiesPath string) usecase.URLMusicRecognizer {
	return &PlatformScraperRecognizer{
		cookiesPath: cookiesPath,
	}
}

type ytdlpDump struct {
	Track       string `json:"track"`
	Artist      string `json:"artist"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (p *PlatformScraperRecognizer) IdentifyByURL(ctx context.Context, url string) (*domain.TrackMetadata, error) {
	args := []string{"--dump-json", "--no-playlist", "--no-warnings"}
	if p.cookiesPath != "" {
		args = append(args, "--cookies", p.cookiesPath)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var dump ytdlpDump
	if err := json.Unmarshal(out, &dump); err != nil {
		return nil, err
	}

	// 1. Check official audio track tag
	if dump.Track != "" {
		artist := dump.Artist
		if artist == "" {
			artist = "Various Artists"
		}
		return &domain.TrackMetadata{
			Title:     dump.Track,
			Artist:    artist,
			IsMatched: true,
		}, nil
	}

	// 2. Fallback: Parse "Artist - Title" patterns from title
	if strings.Contains(dump.Title, " - ") {
		parts := strings.SplitN(dump.Title, " - ", 2)
		return &domain.TrackMetadata{
			Artist:    strings.TrimSpace(parts[0]),
			Title:     strings.TrimSpace(parts[1]),
			IsMatched: true,
		}, nil
	}

	return nil, errors.New("no platform music metadata found")
}
