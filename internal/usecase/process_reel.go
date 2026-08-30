package usecase

import (
	"context"
	"fmt"
	"telegram-audio-bot/internal/domain"
)

type processReelUseCase struct {
	extractor  MediaExtractor
	recognizer MusicRecognizer
	downloader MusicDownloader
}

func NewProcessReelUseCase(e MediaExtractor, r MusicRecognizer, d MusicDownloader) ReelAudioUseCase {
	return &processReelUseCase{
		extractor:  e,
		recognizer: r,
		downloader: d,
	}
}

func (uc *processReelUseCase) Execute(ctx context.Context, targetDir, reelURL string) (*domain.AudioPayload, error) {
	rawPath, snippetPath, err := uc.extractor.ExtractReel(ctx, targetDir, reelURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrExtractionFailed, err)
	}

	meta, err := uc.recognizer.Identify(ctx, snippetPath)
	if err == nil && meta != nil && meta.IsMatched {
		searchQuery := fmt.Sprintf("%s %s audio", meta.Title, meta.Artist)
		fullTrackPath, err := uc.downloader.Download(ctx, targetDir, searchQuery)
		if err == nil && fullTrackPath != "" {
			return &domain.AudioPayload{
				Title:       meta.Title,
				Performer:   meta.Artist,
				FilePath:    fullTrackPath,
				IsFullTrack: true,
			}, nil
		}
	}

	// Fallback response
	return &domain.AudioPayload{
		Title:       "Original Reel Audio",
		Performer:   "Instagram",
		FilePath:    rawPath,
		IsFullTrack: false,
	}, nil
}
