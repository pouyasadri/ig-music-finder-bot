package recognizer

import (
	"context"
	"encoding/json"
	"errors"
	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"

	"github.com/acrcloud/acrcloud_sdk_golang/acrcloud"
)

type ACRCloudRecognizer struct {
	host   string
	key    string
	secret string
}

type acrResponse struct {
	Status struct {
		Code int `json:"code"`
	} `json:"status"`
	Metadata struct {
		Music []struct {
			Title   string `json:"title"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"music"`
	} `json:"metadata"`
}

func NewACRCloudRecognizer(host, key, secret string) usecase.MusicRecognizer {
	return &ACRCloudRecognizer{host: host, key: key, secret: secret}
}

func (a *ACRCloudRecognizer) Identify(ctx context.Context, snippetPath string) (*domain.TrackMetadata, error) {
	client := acrcloud.NewRecognizer(map[string]string{
		"host":           a.host,
		"access_key":     a.key,
		"access_secret":  a.secret,
		"recognize_type": acrcloud.ACR_OPT_REC_AUDIO,
	})

	resStr := client.RecognizeByFile(snippetPath, 0, 10, nil)
	if resStr == "" {
		return nil, errors.New("empty response from acrcloud")
	}

	var res acrResponse
	if err := json.Unmarshal([]byte(resStr), &res); err != nil || res.Status.Code != 0 || len(res.Metadata.Music) == 0 {
		return nil, errors.New("no match found")
	}

	music := res.Metadata.Music[0]
	artist := "Unknown Artist"
	if len(music.Artists) > 0 {
		artist = music.Artists[0].Name
	}

	return &domain.TrackMetadata{
		Title:     music.Title,
		Artist:    artist,
		IsMatched: true,
	}, nil
}
