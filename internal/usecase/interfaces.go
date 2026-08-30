package usecase

import (
	"context"
	"telegram-audio-bot/internal/domain"
)

type MediaExtractor interface {
	ExtractReel(ctx context.Context, targetDir, url string) (rawPath, snippetPath string, err error)
}

type MusicRecognizer interface {
	Identify(ctx context.Context, snippetPath string) (*domain.TrackMetadata, error)
}

type URLMusicRecognizer interface {
	IdentifyByURL(ctx context.Context, url string) (*domain.TrackMetadata, error)
}

type MusicDownloader interface {
	Download(ctx context.Context, targetDir, query string) (filePath, thumbnailPath string, duration int, err error)
}

type ReelAudioUseCase interface {
	Execute(ctx context.Context, targetDir, reelURL string) (*domain.AudioPayload, error)
}
