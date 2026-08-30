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

type ShazamIORecognizer struct {
	scriptPath string
}

func NewShazamIORecognizer(scriptPath string) usecase.MusicRecognizer {
	return &ShazamIORecognizer{scriptPath: scriptPath}
}

type shazamOutput struct {
	Matched    bool   `json:"matched"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	SpotifyURL string `json:"spotify_url"`
	YouTubeURL string `json:"youtube_url"`
	Error      string `json:"error"`
}

func (s *ShazamIORecognizer) Identify(ctx context.Context, snippetPath string) (*domain.TrackMetadata, error) {
	cmd := exec.CommandContext(ctx, "python3", s.scriptPath, snippetPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var res shazamOutput
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &res); err != nil {
		return nil, err
	}

	if !res.Matched || res.Title == "" {
		return nil, errors.New("shazam: no match found")
	}

	return &domain.TrackMetadata{
		Title:      res.Title,
		Artist:     res.Artist,
		IsMatched:  true,
		SpotifyURL: res.SpotifyURL,
		YouTubeURL: res.YouTubeURL,
	}, nil
}
