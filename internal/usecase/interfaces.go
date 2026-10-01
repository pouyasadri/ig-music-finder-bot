package usecase

import (
	"context"

	"telegram-audio-bot/internal/domain"
)

// Application ports used by adapters live in domain. Usecase keeps only
// orchestration service interfaces.

type ReelAudioUseCase interface {
	Execute(ctx context.Context, targetDir, reelURL string, progress ProgressReporter) (*domain.AudioPayload, error)
}

type SoundCloudAudioUseCase interface {
	Execute(ctx context.Context, targetDir, soundCloudURL string, progress ProgressReporter) (*domain.AudioPayload, error)
}
