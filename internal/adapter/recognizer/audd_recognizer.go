package recognizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

type AudDRecognizer struct {
	apiToken string
	client   *http.Client
}

func NewAudDRecognizer(apiToken string, client *http.Client) usecase.MusicRecognizer {
	if client == nil {
		client = http.DefaultClient
	}
	return &AudDRecognizer{
		apiToken: apiToken,
		client:   client,
	}
}

type auddResponse struct {
	Status string `json:"status"`
	Result *struct {
		Artist  string `json:"artist"`
		Title   string `json:"title"`
		Spotify *struct {
			ExternalURLs struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
		} `json:"spotify"`
	} `json:"result"`
	Error *struct {
		ErrorMessage string `json:"error_message"`
	} `json:"error"`
}

func (a *AudDRecognizer) Identify(ctx context.Context, snippetPath string) (*domain.TrackMetadata, error) {
	if a.apiToken == "" {
		return nil, errors.New("audd: missing API token")
	}

	file, err := os.Open(snippetPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("api_token", a.apiToken)
	_ = writer.WriteField("return", "spotify,apple_music")

	part, err := writer.CreateFormFile("file", filepath.Base(snippetPath))
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.audd.io/", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res auddResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Status != "success" || res.Result == nil || res.Result.Title == "" {
		errMsg := "audd: no match found"
		if res.Error != nil && res.Error.ErrorMessage != "" {
			errMsg = "audd error: " + res.Error.ErrorMessage
		}
		return nil, errors.New(errMsg)
	}

	var spotifyURL string
	if res.Result.Spotify != nil {
		spotifyURL = res.Result.Spotify.ExternalURLs.Spotify
	}

	return &domain.TrackMetadata{
		Title:      res.Result.Title,
		Artist:     res.Result.Artist,
		IsMatched:  true,
		SpotifyURL: spotifyURL,
	}, nil
}
