package recognizer

import (
	"context"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

type FallbackRecognizer struct {
	engines []Engine
}

func NewFallbackRecognizer(engines ...Engine) usecase.MusicRecognizer {
	return &FallbackRecognizer{engines: engines}
}

func (f *FallbackRecognizer) Identify(ctx context.Context, snippetPath string) (*domain.TrackMetadata, error) {
	for _, engine := range f.engines {
		slog.Info("attempting music recognition", "engine", engine.Name)

		engineCtx, cancel := context.WithTimeout(ctx, engine.Timeout)
		t0 := time.Now()
		meta, err := engine.Recognizer.Identify(engineCtx, snippetPath)
		cancel()

		elapsed := time.Since(t0).Milliseconds()

		if err == nil && meta != nil && meta.IsMatched {
			slog.Info("recognition successful", "engine", engine.Name, "title", meta.Title, "artist", meta.Artist, "elapsed_ms", elapsed)
			return meta, nil
		}

		slog.Warn("recognition missed or failed, moving to next tier", "engine", engine.Name, "elapsed_ms", elapsed, "err", err)
	}

	return &domain.TrackMetadata{IsMatched: false}, nil
}
