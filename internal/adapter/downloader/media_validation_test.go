package downloader

import (
	"errors"
	"testing"

	"telegram-audio-bot/internal/domain"
)

func TestValidateTrackDuration(t *testing.T) {
	tests := []struct {
		name     string
		actual   int
		expected int
		wantErr  error
	}{
		{name: "matching duration", actual: 240, expected: 240},
		{name: "small provider padding", actual: 250, expected: 240},
		{name: "preview is rejected", actual: 30, expected: 240, wantErr: domain.ErrDurationMismatch},
		{name: "unexpectedly long result is rejected", actual: 1000, expected: 240, wantErr: domain.ErrDurationMismatch},
		{name: "unknown short result is rejected", actual: 30, expected: 0, wantErr: domain.ErrDurationMismatch},
		{name: "unknown normal result is accepted", actual: 180, expected: 0},
		{name: "zero duration is rejected", actual: 0, expected: 180, wantErr: domain.ErrDurationMismatch},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTrackDuration(test.actual, test.expected)
			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("expected valid duration, got %v", err)
				}
				return
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("expected %v, got %v", test.wantErr, err)
			}
		})
	}
}
