package usecase

import (
	"context"
	"strings"

	"telegram-audio-bot/internal/domain"
)

// ProcessMediaInput is the unified input for Instagram, SoundCloud, and uploaded media.
type ProcessMediaInput struct {
	Kind      domain.SourceKind
	SourceURL string
	WorkDir   string
}

// ProcessMediaUseCase orchestrates recognition/download for any supported media source.
type ProcessMediaUseCase interface {
	Execute(ctx context.Context, in ProcessMediaInput, progress ProgressReporter) (*domain.AudioPayload, error)
}

// InferSourceKind derives a source kind from a URL when the caller has not classified it.
func InferSourceKind(raw string) domain.SourceKind {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "soundcloud.com"), strings.Contains(value, "on.soundcloud.com"), strings.Contains(value, "soundcloud.app.goo.gl"):
		return domain.SourceKindSoundCloud
	case strings.HasPrefix(value, "file://"):
		return domain.SourceKindUpload
	default:
		return domain.SourceKindInstagram
	}
}
