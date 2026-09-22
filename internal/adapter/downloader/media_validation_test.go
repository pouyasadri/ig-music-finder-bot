package downloader

import (
	"strings"
	"testing"
)

func TestValidateTrackDuration(t *testing.T) {
	tests := []struct {
		name     string
		actual   int
		expected int
		wantErr  string
	}{
		{name: "matching duration", actual: 240, expected: 240},
		{name: "small provider padding", actual: 250, expected: 240},
		{name: "preview is rejected", actual: 30, expected: 240, wantErr: "shorter than expected"},
		{name: "unexpectedly long result is rejected", actual: 1000, expected: 240, wantErr: "longer than expected"},
		{name: "unknown short result is rejected", actual: 30, expected: 0, wantErr: "preview"},
		{name: "unknown normal result is accepted", actual: 180, expected: 0},
		{name: "zero duration is rejected", actual: 0, expected: 180, wantErr: "unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTrackDuration(test.actual, test.expected)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid duration, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}
