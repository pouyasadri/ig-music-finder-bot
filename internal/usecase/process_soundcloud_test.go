package usecase_test

import (
	"context"
	"errors"
	"testing"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

type mockSoundCloudTrackDownloader struct {
	DownloadTrackFunc func(ctx context.Context, targetDir, url string) (*domain.AudioPayload, error)
}

func (m *mockSoundCloudTrackDownloader) DownloadTrack(ctx context.Context, targetDir, url string) (*domain.AudioPayload, error) {
	return m.DownloadTrackFunc(ctx, targetDir, url)
}

func TestProcessSoundCloudAudio(t *testing.T) {
	ctx := context.Background()
	testDir := "/tmp/test"
	trackURL := "https://soundcloud.com/artist-name/track-name"

	t.Run("success: downloads track directly with metadata and without recognition", func(t *testing.T) {
		var progressMessages []string
		progressCb := func(msg string) {
			progressMessages = append(progressMessages, msg)
		}

		downloader := &mockSoundCloudTrackDownloader{
			DownloadTrackFunc: func(ctx context.Context, dir, url string) (*domain.AudioPayload, error) {
				if dir != testDir || url != trackURL {
					t.Errorf("unexpected args: dir=%s, url=%s", dir, url)
				}
				return &domain.AudioPayload{
					Title:         "Awesome Song",
					Performer:     "Artist 1",
					FilePath:      "/tmp/test/track.mp3",
					ThumbnailPath: "/tmp/test/track.jpg",
					Duration:      195,
				}, nil
			},
		}

		uc := usecase.NewProcessSoundCloudUseCase(downloader)
		payload, err := uc.Execute(ctx, testDir, trackURL, progressCb)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload == nil {
			t.Fatalf("expected non-nil payload")
		}
		if !payload.IsFullTrack {
			t.Errorf("expected IsFullTrack=true, got false")
		}
		if payload.Title != "Awesome Song" {
			t.Errorf("expected title 'Awesome Song', got '%s'", payload.Title)
		}
		if payload.Performer != "Artist 1" {
			t.Errorf("expected performer 'Artist 1', got '%s'", payload.Performer)
		}
		if payload.SoundCloudURL != trackURL {
			t.Errorf("expected soundcloud url '%s', got '%s'", trackURL, payload.SoundCloudURL)
		}
		if payload.Duration != 195 {
			t.Errorf("expected duration 195, got %d", payload.Duration)
		}
		if len(progressMessages) < 2 {
			t.Errorf("expected at least 2 progress messages, got %d", len(progressMessages))
		}
	})

	t.Run("fallback defaults when title and performer empty", func(t *testing.T) {
		downloader := &mockSoundCloudTrackDownloader{
			DownloadTrackFunc: func(ctx context.Context, dir, url string) (*domain.AudioPayload, error) {
				return &domain.AudioPayload{
					FilePath: "/tmp/test/track.mp3",
				}, nil
			},
		}

		uc := usecase.NewProcessSoundCloudUseCase(downloader)
		payload, err := uc.Execute(ctx, testDir, trackURL, nil)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Title != "SoundCloud Audio" {
			t.Errorf("expected default title 'SoundCloud Audio', got '%s'", payload.Title)
		}
		if payload.Performer != "SoundCloud" {
			t.Errorf("expected default performer 'SoundCloud', got '%s'", payload.Performer)
		}
	})

	t.Run("error: downloader failure wraps ErrSoundCloudDownloadFailed", func(t *testing.T) {
		downloader := &mockSoundCloudTrackDownloader{
			DownloadTrackFunc: func(ctx context.Context, dir, url string) (*domain.AudioPayload, error) {
				return nil, errors.New("network failure")
			},
		}

		uc := usecase.NewProcessSoundCloudUseCase(downloader)
		payload, err := uc.Execute(ctx, testDir, trackURL, nil)

		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrSoundCloudDownloadFailed) {
			t.Errorf("expected ErrSoundCloudDownloadFailed, got: %v", err)
		}
		if payload != nil {
			t.Errorf("expected nil payload on error, got: %+v", payload)
		}
	})

	t.Run("error: empty audio path returned wraps ErrSoundCloudDownloadFailed", func(t *testing.T) {
		downloader := &mockSoundCloudTrackDownloader{
			DownloadTrackFunc: func(ctx context.Context, dir, url string) (*domain.AudioPayload, error) {
				return &domain.AudioPayload{FilePath: ""}, nil
			},
		}

		uc := usecase.NewProcessSoundCloudUseCase(downloader)
		_, err := uc.Execute(ctx, testDir, trackURL, nil)

		if err == nil {
			t.Fatalf("expected error for empty file path, got nil")
		}
		if !errors.Is(err, domain.ErrSoundCloudDownloadFailed) {
			t.Errorf("expected ErrSoundCloudDownloadFailed, got: %v", err)
		}
	})
}
