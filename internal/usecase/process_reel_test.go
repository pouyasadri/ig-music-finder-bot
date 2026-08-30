package usecase_test

import (
	"context"
	"errors"
	"testing"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

type mockExtractor struct {
	ExtractFunc func(ctx context.Context, dir, url string) (string, string, error)
}

func (m *mockExtractor) ExtractReel(ctx context.Context, dir, url string) (string, string, error) {
	return m.ExtractFunc(ctx, dir, url)
}

type mockRecognizer struct {
	IdentifyFunc func(ctx context.Context, snippet string) (*domain.TrackMetadata, error)
}

func (m *mockRecognizer) Identify(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
	return m.IdentifyFunc(ctx, snippet)
}

type mockDownloader struct {
	DownloadFunc func(ctx context.Context, dir, query string) (string, string, int, error)
}

func (m *mockDownloader) Download(ctx context.Context, dir, query string) (string, string, int, error) {
	return m.DownloadFunc(ctx, dir, query)
}

func TestProcessReelAudio(t *testing.T) {
	ctx := context.Background()
	testDir := "/tmp/test"

	t.Run("success: recognized and downloaded full track", func(t *testing.T) {
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "/tmp/raw.mp3", "/tmp/snippet.mp3", nil
			},
		}
		recognizer := &mockRecognizer{
			IdentifyFunc: func(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
				return &domain.TrackMetadata{
					Title:      "Song A",
					Artist:     "Artist B",
					IsMatched:  true,
					Duration:   210,
					SpotifyURL: "https://open.spotify.com/track/123",
				}, nil
			},
		}
		downloader := &mockDownloader{
			DownloadFunc: func(ctx context.Context, dir, query string) (string, string, int, error) {
				return "/tmp/full.mp3", "/tmp/cover.jpg", 210, nil
			},
		}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123")

		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if !res.IsFullTrack || res.Title != "Song A" || res.Performer != "Artist B" || res.ThumbnailPath != "/tmp/cover.jpg" || res.Duration != 210 {
			t.Errorf("unexpected output payload: %+v", res)
		}
	})

	t.Run("fallback: unrecognized sound returns raw reel audio", func(t *testing.T) {
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "/tmp/raw.mp3", "/tmp/snippet.mp3", nil
			},
		}
		recognizer := &mockRecognizer{
			IdentifyFunc: func(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
				return nil, errors.New("no fingerprint match")
			},
		}
		downloader := &mockDownloader{}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123")

		if err != nil {
			t.Fatalf("expected successful fallback, got: %v", err)
		}
		if res.IsFullTrack || res.FilePath != "/tmp/raw.mp3" {
			t.Errorf("expected raw fallback audio, got: %+v", res)
		}
	})

	t.Run("fallback: recognized but YouTube download fails", func(t *testing.T) {
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "/tmp/raw.mp3", "/tmp/snippet.mp3", nil
			},
		}
		recognizer := &mockRecognizer{
			IdentifyFunc: func(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
				return &domain.TrackMetadata{Title: "Song A", Artist: "Artist B", IsMatched: true}, nil
			},
		}
		downloader := &mockDownloader{
			DownloadFunc: func(ctx context.Context, dir, query string) (string, string, int, error) {
				return "", "", 0, errors.New("yt download failed")
			},
		}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123")

		if err != nil {
			t.Fatalf("expected fallback on download failure, got: %v", err)
		}
		if res.IsFullTrack || res.FilePath != "/tmp/raw.mp3" {
			t.Errorf("expected raw fallback audio, got: %+v", res)
		}
	})

	t.Run("success: direct YouTube URL routing when available", func(t *testing.T) {
		var downloadedTarget string
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "/tmp/raw.mp3", "/tmp/snippet.mp3", nil
			},
		}
		recognizer := &mockRecognizer{
			IdentifyFunc: func(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
				return &domain.TrackMetadata{
					Title:      "Song Direct",
					Artist:     "Artist Direct",
					IsMatched:  true,
					Duration:   180,
					YouTubeURL: "https://www.youtube.com/watch?v=direct123",
				}, nil
			},
		}
		downloader := &mockDownloader{
			DownloadFunc: func(ctx context.Context, dir, query string) (string, string, int, error) {
				downloadedTarget = query
				return "/tmp/full.mp3", "/tmp/cover.jpg", 0, nil // 0 duration to test fallback to meta.Duration
			},
		}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123")

		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if downloadedTarget != "https://www.youtube.com/watch?v=direct123" {
			t.Errorf("expected direct YouTube URL to be passed to downloader, got: %s", downloadedTarget)
		}
		if res.Duration != 180 {
			t.Errorf("expected fallback to meta.Duration (180), got: %d", res.Duration)
		}
	})

	t.Run("error: extraction failure returns error", func(t *testing.T) {
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "", "", errors.New("network error")
			},
		}
		recognizer := &mockRecognizer{}
		downloader := &mockDownloader{}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123")

		if err == nil {
			t.Fatalf("expected error on extraction failure, got nil")
		}
		if !errors.Is(err, domain.ErrExtractionFailed) {
			t.Errorf("expected ErrExtractionFailed, got: %v", err)
		}
		if res != nil {
			t.Errorf("expected nil result on error, got: %+v", res)
		}
	})
}
