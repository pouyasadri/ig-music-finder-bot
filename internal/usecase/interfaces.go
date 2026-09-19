package usecase

import (
	"context"
	"telegram-audio-bot/internal/domain"
	"time"
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

type MetadataCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error
}

type ReelAudioUseCase interface {
	Execute(ctx context.Context, targetDir, reelURL string, progressCb func(string)) (*domain.AudioPayload, error)
}
