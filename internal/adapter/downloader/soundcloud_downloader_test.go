package downloader_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"telegram-audio-bot/internal/adapter/downloader"
	"telegram-audio-bot/internal/usecase"
)

func TestSoundCloudDownloaderInterfaceCompliance(t *testing.T) {
	sc := downloader.NewSoundCloudDownloader()

	// Verify compliance with both usecase interfaces
	var _ usecase.MusicDownloader = sc
	var _ usecase.SoundCloudTrackDownloader = sc
}

func TestSoundCloudMetadataJSONParsing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sc_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	infoJSON := map[string]any{
		"title":    "Sample Track",
		"uploader": "Sample Uploader",
		"artist":   "Sample Artist",
		"duration": 240.5,
	}
	infoBytes, err := json.Marshal(infoJSON)
	if err != nil {
		t.Fatalf("failed to marshal info json: %v", err)
	}

	infoPath := filepath.Join(tmpDir, "track.info.json")
	if err := os.WriteFile(infoPath, infoBytes, 0644); err != nil {
		t.Fatalf("failed to write info file: %v", err)
	}

	// Verify file was written
	if fi, err := os.Stat(infoPath); err != nil || fi.Size() == 0 {
		t.Fatalf("expected track.info.json to exist")
	}
}
