package downloader_test

import (
	"context"
	"errors"
	"testing"

	"telegram-audio-bot/internal/adapter/downloader"
)

type mockMusicDownloader struct {
	DownloadFunc func(ctx context.Context, targetDir, target string) (string, string, int, error)
}

func (m *mockMusicDownloader) Download(ctx context.Context, targetDir, target string) (string, string, int, error) {
	return m.DownloadFunc(ctx, targetDir, target)
}

func TestFallbackDownloader(t *testing.T) {
	ctx := context.Background()

	t.Run("primary succeeds, fallback not called", func(t *testing.T) {
		fallbackCalled := false
		primary := &mockMusicDownloader{
			DownloadFunc: func(ctx context.Context, targetDir, target string) (string, string, int, error) {
				return "/tmp/sc_track.mp3", "/tmp/sc_thumb.jpg", 180, nil
			},
		}
		secondary := &mockMusicDownloader{
			DownloadFunc: func(ctx context.Context, targetDir, target string) (string, string, int, error) {
				fallbackCalled = true
				return "/tmp/yt_track.mp3", "/tmp/yt_thumb.jpg", 180, nil
			},
		}

		fb := downloader.NewFallbackDownloader(
			downloader.NamedDownloader{Name: "SoundCloud", Downloader: primary},
			downloader.NamedDownloader{Name: "YouTube", Downloader: secondary},
		)

		audio, thumb, dur, err := fb.Download(ctx, "/tmp", "Artist Song")
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if audio != "/tmp/sc_track.mp3" || thumb != "/tmp/sc_thumb.jpg" || dur != 180 {
			t.Errorf("unexpected download results: audio=%s thumb=%s dur=%d", audio, thumb, dur)
		}
		if fallbackCalled {
			t.Errorf("expected fallback not to be called")
		}
	})

	t.Run("primary fails, fallback succeeds", func(t *testing.T) {
		primary := &mockMusicDownloader{
			DownloadFunc: func(ctx context.Context, targetDir, target string) (string, string, int, error) {
				return "", "", 0, errors.New("soundcloud 404")
			},
		}
		secondary := &mockMusicDownloader{
			DownloadFunc: func(ctx context.Context, targetDir, target string) (string, string, int, error) {
				return "/tmp/yt_track.mp3", "/tmp/yt_thumb.jpg", 200, nil
			},
		}

		fb := downloader.NewFallbackDownloader(
			downloader.NamedDownloader{Name: "SoundCloud", Downloader: primary},
			downloader.NamedDownloader{Name: "YouTube", Downloader: secondary},
		)

		audio, thumb, dur, err := fb.Download(ctx, "/tmp", "Artist Song")
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if audio != "/tmp/yt_track.mp3" || thumb != "/tmp/yt_thumb.jpg" || dur != 200 {
			t.Errorf("unexpected download results: audio=%s thumb=%s dur=%d", audio, thumb, dur)
		}
	})

	t.Run("all downloaders fail", func(t *testing.T) {
		primary := &mockMusicDownloader{
			DownloadFunc: func(ctx context.Context, targetDir, target string) (string, string, int, error) {
				return "", "", 0, errors.New("sc err")
			},
		}
		secondary := &mockMusicDownloader{
			DownloadFunc: func(ctx context.Context, targetDir, target string) (string, string, int, error) {
				return "", "", 0, errors.New("yt err")
			},
		}

		fb := downloader.NewFallbackDownloader(
			downloader.NamedDownloader{Name: "SoundCloud", Downloader: primary},
			downloader.NamedDownloader{Name: "YouTube", Downloader: secondary},
		)

		_, _, _, err := fb.Download(ctx, "/tmp", "Artist Song")
		if err == nil {
			t.Fatalf("expected error when all downloaders fail")
		}
	})
}
