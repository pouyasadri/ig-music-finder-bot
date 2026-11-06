package telegram

import (
	"testing"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot/models"
)

func TestClassifyIncoming(t *testing.T) {
	t.Run("instagram url", func(t *testing.T) {
		got, ok := classifyIncoming("see https://www.instagram.com/reel/C-xyz123/", &models.Message{})
		if !ok || got.Kind != domain.SourceKindInstagram || got.SourceURL == "" {
			t.Fatalf("got %+v ok=%v", got, ok)
		}
	})
	t.Run("soundcloud url", func(t *testing.T) {
		got, ok := classifyIncoming("https://soundcloud.com/a/b", &models.Message{})
		if !ok || got.Kind != domain.SourceKindSoundCloud {
			t.Fatalf("got %+v ok=%v", got, ok)
		}
	})
	t.Run("upload media", func(t *testing.T) {
		got, ok := classifyIncoming("", &models.Message{
			Audio: &models.Audio{FileID: "fid", FileUniqueID: "uniq"},
		})
		if !ok || got.Kind != domain.SourceKindUpload || got.SourceURL != "file://uniq" {
			t.Fatalf("got %+v ok=%v", got, ok)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, ok := classifyIncoming("hello", &models.Message{}); ok {
			t.Fatal("expected false")
		}
	})
}
