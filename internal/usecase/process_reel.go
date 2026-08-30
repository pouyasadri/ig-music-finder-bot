package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"telegram-audio-bot/internal/domain"
)

type processReelUseCase struct {
	extractor     MediaExtractor
	recognizer    MusicRecognizer
	urlRecognizer URLMusicRecognizer
	downloader    MusicDownloader
}

func NewProcessReelUseCase(e MediaExtractor, r MusicRecognizer, u URLMusicRecognizer, d MusicDownloader) ReelAudioUseCase {
	return &processReelUseCase{
		extractor:     e,
		recognizer:    r,
		urlRecognizer: u,
		downloader:    d,
	}
}

func (uc *processReelUseCase) Execute(ctx context.Context, targetDir, reelURL string) (*domain.AudioPayload, error) {
	t0 := time.Now()
	rawPath, snippetPath, err := uc.extractor.ExtractReel(ctx, targetDir, reelURL)
	extractMs := time.Since(t0).Milliseconds()
	if err != nil {
		slog.Error("media extraction failed", "extract_ms", extractMs, "err", err)
		return nil, fmt.Errorf("%w: %v", domain.ErrExtractionFailed, err)
	}

	t1 := time.Now()
	meta, err := uc.recognizer.Identify(ctx, snippetPath)
	recognizeMs := time.Since(t1).Milliseconds()

	// Tier 4 fallback if primary recognizers miss
	if (err != nil || meta == nil || !meta.IsMatched) && uc.urlRecognizer != nil {
		tUrl := time.Now()
		urlCtx, cancelUrl := context.WithTimeout(ctx, 3*time.Second)
		urlMeta, urlErr := uc.urlRecognizer.IdentifyByURL(urlCtx, reelURL)
		cancelUrl()
		
		if urlErr == nil && urlMeta != nil && urlMeta.IsMatched {
			meta = urlMeta
			err = nil
			slog.Info("url metadata scraping matched", "url_recognize_ms", time.Since(tUrl).Milliseconds())
		}
	}

	if err == nil && meta != nil && meta.IsMatched {
		slog.Info("track recognized", "title", meta.Title, "artist", meta.Artist, "extract_ms", extractMs, "recognize_ms", recognizeMs)

		// Prefer direct YouTube URL if ACRCloud provided it, avoiding expensive search queries
		downloadTarget := meta.YouTubeURL
		if downloadTarget == "" {
			downloadTarget = fmt.Sprintf("%s %s audio", meta.Title, meta.Artist)
		}

		t2 := time.Now()
		fullTrackPath, thumbnailPath, duration, err := uc.downloader.Download(ctx, targetDir, downloadTarget)
		downloadMs := time.Since(t2).Milliseconds()

		if err == nil && fullTrackPath != "" {
			slog.Info("download completed", "download_ms", downloadMs, "target", downloadTarget)

			finalDuration := duration
			if finalDuration <= 0 {
				finalDuration = meta.Duration
			}

			return &domain.AudioPayload{
				Title:         meta.Title,
				Performer:     meta.Artist,
				FilePath:      fullTrackPath,
				ThumbnailPath: thumbnailPath,
				Duration:      finalDuration,
				IsFullTrack:   true,
				SpotifyURL:    meta.SpotifyURL,
				YouTubeURL:    meta.YouTubeURL,
			}, nil
		}
		slog.Warn("download failed, falling back to raw reel audio", "download_ms", downloadMs, "err", err)
	} else {
		slog.Info("unrecognized sound, using raw reel audio", "extract_ms", extractMs, "recognize_ms", recognizeMs)
	}

	// Fallback response
	return &domain.AudioPayload{
		Title:       "Original Reel Audio",
		Performer:   "Instagram",
		FilePath:    rawPath,
		IsFullTrack: false,
	}, nil
}
