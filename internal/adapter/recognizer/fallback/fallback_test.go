package fallback_test

import (
	"context"
	"testing"
	"time"

	"telegram-audio-bot/internal/adapter/recognizer/fallback"
	"telegram-audio-bot/internal/domain"
)

type stubRecognizer struct {
	meta  *domain.TrackMetadata
	err   error
	calls *int
}

func (s stubRecognizer) Identify(context.Context, string) (*domain.TrackMetadata, error) {
	if s.calls != nil {
		*s.calls++
	}
	return s.meta, s.err
}

func TestShortCircuit(t *testing.T) {
	firstCalls, secondCalls := 0, 0
	strong := &domain.TrackMetadata{
		Title:      "Strong Hit",
		Artist:     "Artist",
		Duration:   200,
		SpotifyURL: "https://open.spotify.com/track/1",
		IsMatched:  true,
	}
	rec := fallback.New(
		fallback.Engine{Name: "first", Recognizer: stubRecognizer{meta: strong, calls: &firstCalls}, Timeout: time.Second},
		fallback.Engine{Name: "second", Recognizer: stubRecognizer{meta: strong, calls: &secondCalls}, Timeout: time.Second},
	)

	meta, err := rec.Identify(context.Background(), "snippet.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil || meta.Title != "Strong Hit" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
	if firstCalls != 1 {
		t.Fatalf("first engine calls = %d, want 1", firstCalls)
	}
	if secondCalls != 0 {
		t.Fatalf("second engine calls = %d, want 0 (short-circuit)", secondCalls)
	}
}

func TestWithoutShortCircuitRunsAll(t *testing.T) {
	firstCalls, secondCalls := 0, 0
	weak := &domain.TrackMetadata{Title: "Only Title", IsMatched: true}
	rec := fallback.New(
		fallback.Engine{Name: "first", Recognizer: stubRecognizer{meta: weak, calls: &firstCalls}, Timeout: time.Second},
		fallback.Engine{Name: "second", Recognizer: stubRecognizer{meta: weak, calls: &secondCalls}, Timeout: time.Second},
	).WithScores(0.3, 0)

	meta, err := rec.Identify(context.Background(), "snippet.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil || !meta.IsMatched {
		t.Fatalf("expected match, got %+v", meta)
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("expected both engines, got first=%d second=%d", firstCalls, secondCalls)
	}
}
