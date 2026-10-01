package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

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
	calls        int
}

func (m *mockRecognizer) Identify(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
	m.calls++
	return m.IdentifyFunc(ctx, snippet)
}

type mockDownloader struct {
	DownloadFunc func(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error)
}

func (m *mockDownloader) Download(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error) {
	return m.DownloadFunc(ctx, dir, query, expectedDuration)
}

type mockMetadataCache struct {
	data map[string][]byte
}

func (m *mockMetadataCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.data == nil {
		return nil, errors.New("miss")
	}
	value, ok := m.data[key]
	if !ok {
		return nil, errors.New("miss")
	}
	return value, nil
}

func (m *mockMetadataCache) Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error {
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[key] = append([]byte(nil), value...)
	return nil
}

type recordingProgress struct {
	stages []domain.ProgressStage
}

func (r *recordingProgress) Report(stage domain.ProgressStage, detail usecase.ProgressDetail) {
	r.stages = append(r.stages, stage)
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
			DownloadFunc: func(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error) {
				return "/tmp/full.mp3", "/tmp/cover.jpg", 210, nil
			},
		}

		progress := &recordingProgress{}
		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123", progress)

		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if !res.IsFullTrack || res.Title != "Song A" || res.Performer != "Artist B" || res.ThumbnailPath != "/tmp/cover.jpg" || res.Duration != 210 {
			t.Errorf("unexpected output payload: %+v", res)
		}
		if len(progress.stages) < 3 {
			t.Fatalf("expected progress stages, got %v", progress.stages)
		}
		if progress.stages[0] != domain.ProgressExtracting || progress.stages[1] != domain.ProgressRecognizing {
			t.Fatalf("unexpected early stages: %v", progress.stages)
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
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123", nil)

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
			DownloadFunc: func(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error) {
				return "", "", 0, errors.New("yt download failed")
			},
		}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123", nil)

		if err != nil {
			t.Fatalf("expected fallback on download failure, got: %v", err)
		}
		if res.IsFullTrack || res.FilePath != "/tmp/raw.mp3" {
			t.Errorf("expected raw fallback audio, got: %+v", res)
		}
	})

	t.Run("fallback: short download is not sent as a full track", func(t *testing.T) {
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "/tmp/raw.mp3", "/tmp/snippet.mp3", nil
			},
		}
		recognizer := &mockRecognizer{
			IdentifyFunc: func(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
				return &domain.TrackMetadata{Title: "Song A", Artist: "Artist B", IsMatched: true, Duration: 240}, nil
			},
		}
		downloader := &mockDownloader{
			DownloadFunc: func(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error) {
				return "/tmp/preview.mp3", "", 30, nil
			},
		}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123", nil)

		if err != nil {
			t.Fatalf("expected raw fallback, got: %v", err)
		}
		if res.IsFullTrack || res.FilePath != "/tmp/raw.mp3" {
			t.Errorf("expected short download to be rejected, got: %+v", res)
		}
	})

	t.Run("success: clean title and artist passed to downloader", func(t *testing.T) {
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
			DownloadFunc: func(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error) {
				downloadedTarget = query
				return "/tmp/full.mp3", "/tmp/cover.jpg", 180, nil
			},
		}

		uc := usecase.NewProcessReelUseCase(extractor, recognizer, nil, downloader)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123", nil)

		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if downloadedTarget != "Song Direct Artist Direct" {
			t.Errorf("expected clean title and artist query, got: %s", downloadedTarget)
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
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/123", nil)

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

	t.Run("cache decorator reuses recognition metadata", func(t *testing.T) {
		meta := domain.TrackMetadata{Title: "Cached", Artist: "Artist", IsMatched: true, Duration: 120}
		encoded, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		cache := &mockMetadataCache{data: map[string][]byte{
			domain.NormalizeMediaURL("https://instagram.com/reel/cached"): encoded,
		}}
		recognizer := &mockRecognizer{
			IdentifyFunc: func(ctx context.Context, snippet string) (*domain.TrackMetadata, error) {
				t.Fatal("recognizer should not be called on cache hit")
				return nil, nil
			},
		}
		extractor := &mockExtractor{
			ExtractFunc: func(ctx context.Context, dir, url string) (string, string, error) {
				return "/tmp/raw.mp3", "/tmp/snippet.mp3", nil
			},
		}
		downloader := &mockDownloader{
			DownloadFunc: func(ctx context.Context, dir, query string, expectedDuration int) (string, string, int, error) {
				return "/tmp/full.mp3", "", 120, nil
			},
		}

		uc := usecase.NewCachedProcessReelUseCase(extractor, recognizer, nil, downloader, cache, time.Hour)
		res, err := uc.Execute(ctx, testDir, "https://instagram.com/reel/cached", usecase.NopProgress())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Title != "Cached" || !res.IsFullTrack {
			t.Fatalf("unexpected payload: %+v", res)
		}
		if recognizer.calls != 0 {
			t.Fatalf("expected 0 recognizer calls, got %d", recognizer.calls)
		}
	})
}
